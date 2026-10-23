package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

var ErrMCPUnavailable = errors.New("MCP server unavailable")

type MCPClient struct {
	token string
	traceEnabled bool
	http  *http.Client
}

type MCPTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"`
}

func NewMCPClient(token string, traceEnabled ...bool) *MCPClient {
	enabled := len(traceEnabled) > 0 && traceEnabled[0]
	return &MCPClient{token: strings.TrimSpace(token), traceEnabled: enabled, http: &http.Client{Timeout: 5 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}}
}

func (client *MCPClient) Configured() bool { return strings.TrimSpace(client.token) != "" }

func (client *MCPClient) Discover(ctx context.Context, baseURL string) ([]MCPTool, error) {
	if !client.Configured() {
		return nil, fmt.Errorf("MCP authentication is not configured: %w", ErrMCPUnavailable)
	}
	var initialized struct {
		ProtocolVersion string `json:"protocolVersion"`
		Capabilities    struct {
			Tools json.RawMessage `json:"tools"`
		} `json:"capabilities"`
		ServerInfo struct {
			Name    string `json:"name"`
			Version string `json:"version"`
		} `json:"serverInfo"`
	}
	if err := client.call(ctx, baseURL, "initialize", map[string]any{"protocolVersion": "2024-11-05", "capabilities": map[string]any{}, "clientInfo": map[string]string{"name": "campusclaw-api", "version": "1.0.0"}}, &initialized); err != nil {
		return nil, err
	}
	if initialized.ProtocolVersion == "" || initialized.ServerInfo.Name == "" {
		return nil, fmt.Errorf("invalid MCP initialize response: %w", ErrMCPUnavailable)
	}
	var listed struct {
		Tools []MCPTool `json:"tools"`
	}
	if err := client.call(ctx, baseURL, "tools/list", map[string]any{}, &listed); err != nil {
		return nil, err
	}
	if len(listed.Tools) == 0 || len(listed.Tools) > 100 {
		return nil, fmt.Errorf("invalid MCP tools/list response: %w", ErrMCPUnavailable)
	}
	seen := map[string]bool{}
	for _, tool := range listed.Tools {
		var schema struct {
			Type string `json:"type"`
		}
		if len(tool.Name) < 1 || len(tool.Name) > 120 || tool.Description == "" || len(tool.Description) > 1000 || seen[tool.Name] || json.Unmarshal(tool.InputSchema, &schema) != nil || schema.Type != "object" {
			return nil, fmt.Errorf("invalid MCP tool descriptor: %w", ErrMCPUnavailable)
		}
		seen[tool.Name] = true
	}
	return listed.Tools, nil
}

func (client *MCPClient) Call(ctx context.Context, baseURL, name string, arguments map[string]any) (string, error) {
	if !client.Configured() {
		return "", fmt.Errorf("MCP authentication is not configured: %w", ErrMCPUnavailable)
	}
	var result struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}
	if err := client.call(ctx, baseURL, "tools/call", map[string]any{"name": name, "arguments": arguments}, &result); err != nil {
		return "", err
	}
	if result.IsError || len(result.Content) == 0 || result.Content[0].Type != "text" || strings.TrimSpace(result.Content[0].Text) == "" || len(result.Content[0].Text) > 4*1024 {
		return "", fmt.Errorf("invalid MCP tool result: %w", ErrMCPUnavailable)
	}
	return result.Content[0].Text, nil
}

func (client *MCPClient) call(ctx context.Context, baseURL, method string, params, output any) error {
	body, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": "campusclaw-1", "method": method, "params": params})
	if err != nil || len(body) > 64*1024 {
		return fmt.Errorf("invalid MCP request: %w", ErrMCPUnavailable)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(baseURL, "/")+"/mcp", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("invalid MCP URL: %w", ErrMCPUnavailable)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Authorization", "Bearer "+client.token)
	if client.traceEnabled && strings.HasPrefix(baseURL, "http://mcp-demo:") {
		if scenario, ok := ctx.Value(testScenarioContextKey).(string); ok && testScenarioPattern.MatchString(scenario) { request.Header.Set("X-CampusClaw-Test-Scenario", scenario) }
	}
	response, err := client.http.Do(request)
	if err != nil {
		return fmt.Errorf("MCP request failed: %w", ErrMCPUnavailable)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 1024))
		return fmt.Errorf("MCP HTTP %d: %w", response.StatusCode, ErrMCPUnavailable)
	}
	var envelope struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      string          `json:"id"`
		Result  json.RawMessage `json:"result"`
		Error   *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 64*1024)).Decode(&envelope); err != nil || envelope.JSONRPC != "2.0" || envelope.ID != "campusclaw-1" {
		return fmt.Errorf("invalid MCP JSON-RPC response: %w", ErrMCPUnavailable)
	}
	if envelope.Error != nil {
		return fmt.Errorf("MCP JSON-RPC error %d: %w", envelope.Error.Code, ErrMCPUnavailable)
	}
	if len(envelope.Result) == 0 {
		return fmt.Errorf("MCP response has no result: %w", ErrMCPUnavailable)
	}
	return json.Unmarshal(envelope.Result, output)
}
