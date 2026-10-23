package app

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMCPClientDiscoveryAndCall(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-only-token" { t.Errorf("authorization header missing") }
		var request struct { JSONRPC string `json:"jsonrpc"`; ID string `json:"id"`; Method string `json:"method"`; Params map[string]any `json:"params"` }
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil { t.Fatal(err) }
		var result any
		switch request.Method {
		case "initialize": result = map[string]any{"protocolVersion": "2024-11-05", "capabilities": map[string]any{"tools": map[string]any{}}, "serverInfo": map[string]any{"name": "test", "version": "1"}}
		case "tools/list": result = map[string]any{"tools": []any{map[string]any{"name": "formula_hint", "description": "hint", "inputSchema": map[string]any{"type": "object"}}}}
		case "tools/call": result = map[string]any{"content": []any{map[string]any{"type": "text", "text": "hint"}}, "isError": false}
		default: t.Fatalf("unexpected method %s", request.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": request.JSONRPC, "id": request.ID, "result": result})
	}))
	defer server.Close()
	client := NewMCPClient("test-only-token")
	tools, err := client.Discover(context.Background(), server.URL)
	if err != nil || len(tools) != 1 || tools[0].Name != "formula_hint" { t.Fatalf("Discover() tools=%v err=%v", tools, err) }
	text, err := client.Call(context.Background(), server.URL, "formula_hint", map[string]any{"topic": "speed"})
	if err != nil || text != "hint" { t.Fatalf("Call() text=%q err=%v", text, err) }
}

func TestMCPClientFailsClosedWithoutToken(t *testing.T) {
	client := NewMCPClient("")
	if _, err := client.Discover(context.Background(), "http://mcp-demo:8090"); err == nil { t.Fatal("Discover accepted an empty MCP token") }
	if _, err := client.Call(context.Background(), "http://mcp-demo:8090", "formula_hint", map[string]any{}); err == nil { t.Fatal("Call accepted an empty MCP token") }
}

func TestBoundToolSelectionIsExplicitAndLimited(t *testing.T) {
	bindings := []MCPToolBinding{{Ref: "unit_convert@mcp-demo"}, {Ref: "formula_hint@mcp-demo"}}
	selected := selectBoundMCPCalls("把 1.5 km 换算成 m，并给出速度公式提示", bindings)
	if len(selected) != 2 { t.Fatalf("selected %d calls, want two", len(selected)) }
	if selected[0].ref != "unit_convert@mcp-demo" || selected[1].ref != "formula_hint@mcp-demo" { t.Fatalf("unexpected selections: %#v", selected) }
	if len(selectBoundMCPCalls("实验前先做什么准备？", bindings)) != 0 { t.Fatal("unrelated question triggered an MCP tool") }
	tooMany := []MCPToolBinding{{Ref: "unit_convert@a"}, {Ref: "unit_convert@b"}, {Ref: "unit_convert@c"}, {Ref: "unit_convert@d"}}
	if got := selectBoundMCPCalls("1 km 转 m", tooMany); len(got) > maxMCPCallsPerMessage { t.Fatalf("tool call quota exceeded: %d", len(got)) }
}

func TestValidToolReference(t *testing.T) {
	for _, ref := range []string{"unit_convert@mcp-demo", "formula_hint@server_2"} { if !validToolRef(ref) { t.Errorf("valid reference rejected: %q", ref) } }
	for _, ref := range []string{"unit_convert", "unit_convert@a@b", "../tool@mcp-demo", "unit_convert@bad host"} { if validToolRef(ref) { t.Errorf("invalid reference accepted: %q", ref) } }
}
