package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"
)

var mcpNamePattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]{0,39}$`)
var mcpToolNamePattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]{0,119}$`)

func validToolRef(value string) bool {
	parts := strings.Split(value, "@")
	return len(parts) == 2 && mcpToolNamePattern.MatchString(parts[0]) && mcpNamePattern.MatchString(parts[1])
}
func (s *Server) listMCPServers(w http.ResponseWriter, r *http.Request) {
	items, err := s.store.ListMCPServers(r.Context(), currentUser(r.Context()).ClassID, s.mcp.Configured())
	if err != nil { writeError(w, http.StatusInternalServerError, "could not list MCP servers"); return }
	writeJSON(w, http.StatusOK, map[string]any{"servers": items})
}

type mcpServerRequest struct { Name string `json:"name"`; BaseURL string `json:"base_url"` }

func (s *Server) validateMCPServerRequest(w http.ResponseWriter, request mcpServerRequest) bool {
	request.Name = strings.TrimSpace(request.Name)
	if !mcpNamePattern.MatchString(request.Name) || strings.TrimRight(strings.TrimSpace(request.BaseURL), "/") != s.config.MCPAllowedServerURL {
		writeError(w, http.StatusBadRequest, "name is invalid or base_url is not an allowed internal MCP service")
		return false
	}
	return true
}

func (s *Server) createMCPServer(w http.ResponseWriter, r *http.Request) {
	var request mcpServerRequest
	if err := decodeJSON(r, &request); err != nil { writeError(w, http.StatusBadRequest, "invalid MCP server request"); return }
	request.Name = strings.TrimSpace(request.Name)
	if !s.validateMCPServerRequest(w, request) { return }
	user := currentUser(r.Context())
	item, err := s.store.CreateMCPServer(r.Context(), user.ClassID, user.ID, request.Name, s.config.MCPAllowedServerURL, s.mcp.Configured())
	if err != nil { writeError(w, http.StatusConflict, "could not register MCP server"); return }
	writeJSON(w, http.StatusCreated, item)
}

func (s *Server) updateMCPServer(w http.ResponseWriter, r *http.Request) {
	id, ok := requestID(w, r); if !ok { return }
	var request mcpServerRequest
	if err := decodeJSON(r, &request); err != nil { writeError(w, http.StatusBadRequest, "invalid MCP server request"); return }
	request.Name = strings.TrimSpace(request.Name)
	if !s.validateMCPServerRequest(w, request) { return }
	item, err := s.store.UpdateMCPServer(r.Context(), id, currentUser(r.Context()).ClassID, request.Name, s.mcp.Configured())
	if errors.Is(err, ErrNotFound) { writeError(w, http.StatusNotFound, "MCP server not found"); return }
	if err != nil { writeError(w, http.StatusConflict, "disconnect the server before editing it"); return }
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) deleteMCPServer(w http.ResponseWriter, r *http.Request) {
	id, ok := requestID(w, r); if !ok { return }
	err := s.store.DeleteMCPServer(r.Context(), id, currentUser(r.Context()).ClassID)
	if errors.Is(err, ErrNotFound) { writeError(w, http.StatusNotFound, "MCP server not found"); return }
	if err != nil { writeError(w, http.StatusConflict, "disconnect the server and remove skill bindings before deleting it"); return }
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) connectMCPServer(w http.ResponseWriter, r *http.Request) {
	id, ok := requestID(w, r); if !ok { return }
	if !s.mcp.Configured() { writeError(w, http.StatusServiceUnavailable, "MCP authentication is not configured"); return }
	user := currentUser(r.Context())
	server, err := s.store.MCPServer(r.Context(), id, user.ClassID, false)
	if errors.Is(err, ErrNotFound) { writeError(w, http.StatusNotFound, "MCP server not found"); return }
	if err != nil { writeError(w, http.StatusInternalServerError, "could not load MCP server"); return }
	if err := s.store.DisconnectMCPServer(r.Context(), id, user.ClassID); err != nil { writeError(w, http.StatusInternalServerError, "could not clear old discovery cache"); return }
	ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second); defer cancel()
	tools, err := s.mcp.Discover(ctx, server.BaseURL)
	if err != nil { writeError(w, http.StatusServiceUnavailable, "MCP initialize or tools/list failed; server remains disconnected"); return }
	if err := s.store.CacheMCPTools(r.Context(), id, user.ClassID, tools); err != nil { writeError(w, http.StatusInternalServerError, "could not cache discovered tools"); return }
	writeJSON(w, http.StatusOK, map[string]any{"server_id": id, "connected": true, "tools_discovered": len(tools), "has_token": true})
}

func (s *Server) disconnectMCPServer(w http.ResponseWriter, r *http.Request) {
	id, ok := requestID(w, r); if !ok { return }
	if err := s.store.DisconnectMCPServer(r.Context(), id, currentUser(r.Context()).ClassID); errors.Is(err, ErrNotFound) { writeError(w, http.StatusNotFound, "MCP server not found"); return } else if err != nil { writeError(w, http.StatusInternalServerError, "could not disconnect MCP server"); return }
	writeJSON(w, http.StatusOK, map[string]any{"server_id": id, "connected": false, "tools_cleared": true})
}

func (s *Server) listMCPTools(w http.ResponseWriter, r *http.Request) {
	id, ok := requestID(w, r); if !ok { return }
	items, err := s.store.ListMCPTools(r.Context(), id, currentUser(r.Context()).ClassID)
	if errors.Is(err, ErrNotFound) { writeError(w, http.StatusNotFound, "connected MCP server not found"); return }
	if err != nil { writeError(w, http.StatusInternalServerError, "could not list discovered tools"); return }
	writeJSON(w, http.StatusOK, map[string]any{"tools": items})
}

func (s *Server) listMCPToolCalls(w http.ResponseWriter, r *http.Request) {
	items, err := s.store.ListMCPAudit(r.Context(), currentUser(r.Context()).ClassID)
	if err != nil { writeError(w, http.StatusInternalServerError, "could not list MCP audit records"); return }
	writeJSON(w, http.StatusOK, map[string]any{"tool_calls": items})
}

func (s *Store) ListMCPServers(ctx context.Context, classID int64, hasToken bool) ([]MCPServer, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT id, name, base_url, is_connected, created_at FROM mcp_servers WHERE class_id=? ORDER BY id`, classID)
	if err != nil { return nil, err }; defer rows.Close()
	items := []MCPServer{}
	for rows.Next() { var item MCPServer; if err := rows.Scan(&item.ID, &item.Name, &item.BaseURL, &item.Connected, &item.CreatedAt); err != nil { return nil, err }; item.HasToken = hasToken; items = append(items, item) }
	return items, rows.Err()
}
func (s *Store) CreateMCPServer(ctx context.Context, classID, userID int64, name, baseURL string, hasToken bool) (MCPServer, error) {
	result, err := s.DB.ExecContext(ctx, `INSERT INTO mcp_servers (class_id, created_by, name, base_url) VALUES (?, ?, ?, ?)`, classID, userID, name, baseURL)
	if err != nil { return MCPServer{}, err }
	id, err := result.LastInsertId(); if err != nil { return MCPServer{}, err }
	return s.MCPServer(ctx, id, classID, hasToken)
}

func (s *Store) MCPServer(ctx context.Context, id, classID int64, hasToken bool) (MCPServer, error) {
	var item MCPServer
	err := s.DB.QueryRowContext(ctx, `SELECT id, name, base_url, is_connected, created_at FROM mcp_servers WHERE id=? AND class_id=?`, id, classID).Scan(&item.ID, &item.Name, &item.BaseURL, &item.Connected, &item.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) { return MCPServer{}, ErrNotFound }; item.HasToken = hasToken; return item, err
}

func (s *Store) MCPServerByName(ctx context.Context, name string, classID int64) (MCPServer, error) {
	var item MCPServer
	err := s.DB.QueryRowContext(ctx, `SELECT id, name, base_url, is_connected, created_at FROM mcp_servers WHERE name=? AND class_id=?`, name, classID).Scan(&item.ID, &item.Name, &item.BaseURL, &item.Connected, &item.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) { return MCPServer{}, ErrNotFound }
	return item, err
}

func (s *Store) UpdateMCPServer(ctx context.Context, id, classID int64, name string, hasToken bool) (MCPServer, error) {
	result, err := s.DB.ExecContext(ctx, `UPDATE mcp_servers SET name=? WHERE id=? AND class_id=? AND is_connected=FALSE`, name, id, classID)
	if err != nil { return MCPServer{}, err }
	changed, err := result.RowsAffected(); if err != nil { return MCPServer{}, err }
	if changed == 0 { var connected bool; err := s.DB.QueryRowContext(ctx, `SELECT is_connected FROM mcp_servers WHERE id=? AND class_id=?`, id, classID).Scan(&connected); if errors.Is(err, sql.ErrNoRows) { return MCPServer{}, ErrNotFound }; if err != nil { return MCPServer{}, err }; if connected { return MCPServer{}, fmt.Errorf("connected") } }
	return s.MCPServer(ctx, id, classID, hasToken)
}

func (s *Store) DeleteMCPServer(ctx context.Context, id, classID int64) error {
	var connected bool
	err := s.DB.QueryRowContext(ctx, `SELECT is_connected FROM mcp_servers WHERE id=? AND class_id=?`, id, classID).Scan(&connected)
	if errors.Is(err, sql.ErrNoRows) { return ErrNotFound }; if err != nil { return err }; if connected { return fmt.Errorf("connected") }
	serverName, err := s.MCPServerByNameForID(ctx, id, classID); if err != nil { return err }
	rows, err := s.DB.QueryContext(ctx, `SELECT tools FROM teacher_skills WHERE class_id=?`, classID); if err != nil { return err }
	for rows.Next() { var raw []byte; if err := rows.Scan(&raw); err != nil { rows.Close(); return err }; var bindings []MCPToolBinding; _ = json.Unmarshal(raw, &bindings); for _, binding := range bindings { if strings.HasSuffix(binding.Ref, "@"+serverName) { rows.Close(); return fmt.Errorf("server has skill bindings") } } }
	if err := rows.Err(); err != nil { rows.Close(); return err }; rows.Close()
	_, err = s.DB.ExecContext(ctx, `DELETE FROM mcp_servers WHERE id=? AND class_id=? AND is_connected=FALSE`, id, classID)
	return err
}

func (s *Store) MCPServerByNameForID(ctx context.Context, id, classID int64) (string, error) {
	var name string
	err := s.DB.QueryRowContext(ctx, `SELECT name FROM mcp_servers WHERE id=? AND class_id=?`, id, classID).Scan(&name)
	if errors.Is(err, sql.ErrNoRows) { return "", ErrNotFound }; return name, err
}

func (s *Store) DisconnectMCPServer(ctx context.Context, id, classID int64) error {
	result, err := s.DB.ExecContext(ctx, `UPDATE mcp_servers SET is_connected=FALSE WHERE id=? AND class_id=?`, id, classID); if err != nil { return err }
	changed, err := result.RowsAffected(); if err != nil { return err }
	var exists int
	if changed == 0 { if err := s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM mcp_servers WHERE id=? AND class_id=?`, id, classID).Scan(&exists); err != nil { return err }; if exists == 0 { return ErrNotFound } }
	_, err = s.DB.ExecContext(ctx, `DELETE FROM mcp_server_tools WHERE server_id=?`, id); return err
}

func (s *Store) CacheMCPTools(ctx context.Context, id, classID int64, tools []MCPTool) error {
	tx, err := s.DB.BeginTx(ctx, nil); if err != nil { return err }; defer tx.Rollback()
	var exists int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM mcp_servers WHERE id=? AND class_id=?`, id, classID).Scan(&exists); err != nil { return err }; if exists == 0 { return ErrNotFound }
	if _, err := tx.ExecContext(ctx, `DELETE FROM mcp_server_tools WHERE server_id=?`, id); err != nil { return err }
	for _, tool := range tools { if !json.Valid(tool.InputSchema) { return fmt.Errorf("invalid input schema") }; if _, err := tx.ExecContext(ctx, `INSERT INTO mcp_server_tools (server_id, name, description, input_schema) VALUES (?, ?, ?, ?)`, id, tool.Name, tool.Description, string(tool.InputSchema)); err != nil { return err } }
	if _, err := tx.ExecContext(ctx, `UPDATE mcp_servers SET is_connected=TRUE WHERE id=? AND class_id=?`, id, classID); err != nil { return err }
	return tx.Commit()
}

func (s *Store) ListMCPTools(ctx context.Context, id, classID int64) ([]MCPDiscoveredTool, error) {
	var connected bool
	err := s.DB.QueryRowContext(ctx, `SELECT is_connected FROM mcp_servers WHERE id=? AND class_id=?`, id, classID).Scan(&connected)
	if errors.Is(err, sql.ErrNoRows) { return nil, ErrNotFound }; if err != nil { return nil, err }; if !connected { return nil, ErrNotFound }
	rows, err := s.DB.QueryContext(ctx, `SELECT name, description, input_schema, discovered_at FROM mcp_server_tools WHERE server_id=? ORDER BY name`, id); if err != nil { return nil, err }; defer rows.Close()
	items := []MCPDiscoveredTool{}
	for rows.Next() { var item MCPDiscoveredTool; var schema []byte; if err := rows.Scan(&item.Name, &item.Description, &schema, &item.DiscoveredAt); err != nil { return nil, err }; item.InputSchema = json.RawMessage(schema); items = append(items, item) }
	return items, rows.Err()
}

func (s *Store) ValidateToolBinding(ctx context.Context, classID int64, ref string) error {
	parts := strings.Split(ref, "@"); if len(parts) != 2 { return ErrNotFound }
	var count int
	err := s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM mcp_servers s JOIN mcp_server_tools t ON t.server_id=s.id WHERE s.class_id=? AND s.name=? AND s.is_connected=TRUE AND t.name=?`, classID, parts[1], parts[0]).Scan(&count)
	if err != nil { return err }; if count != 1 { return ErrNotFound }; return nil
}

func (s *Store) ResolveMCPBinding(ctx context.Context, classID int64, ref string) (MCPServer, MCPTool, error) {
	parts := strings.Split(ref, "@"); if len(parts) != 2 { return MCPServer{}, MCPTool{}, ErrNotFound }
	var server MCPServer; var tool MCPTool; var schema []byte
	err := s.DB.QueryRowContext(ctx, `SELECT s.id, s.name, s.base_url, s.is_connected, t.name, t.description, t.input_schema FROM mcp_servers s JOIN mcp_server_tools t ON t.server_id=s.id WHERE s.class_id=? AND s.name=? AND s.is_connected=TRUE AND t.name=?`, classID, parts[1], parts[0]).Scan(&server.ID, &server.Name, &server.BaseURL, &server.Connected, &tool.Name, &tool.Description, &schema)
	if errors.Is(err, sql.ErrNoRows) { return MCPServer{}, MCPTool{}, ErrNotFound }; if err != nil { return MCPServer{}, MCPTool{}, err }; tool.InputSchema = json.RawMessage(schema); return server, tool, nil
}

func (s *Store) StartMCPAudit(ctx context.Context, classID int64, serverID *int64, userID, sessionID, messageID int64, serverName, toolName, status, errorCode string) (int64, error) {
	var code any; if errorCode != "" { code = errorCode }
	var server any; if serverID != nil { server = *serverID }
	result, err := s.DB.ExecContext(ctx, `INSERT INTO mcp_tool_call_audit (class_id, server_id, server_name, tool_name, user_id, session_id, message_id, status, error_code) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, classID, server, serverName, toolName, userID, sessionID, messageID, status, code)
	if err != nil { return 0, err }; return result.LastInsertId()
}

func (s *Store) FinishMCPAudit(ctx context.Context, auditID int64, messageID int64, status, errorCode string) error {
	var code any; if errorCode != "" { code = errorCode }
	_, err := s.DB.ExecContext(ctx, `UPDATE mcp_tool_call_audit SET message_id=?, status=?, error_code=? WHERE id=?`, messageID, status, code, auditID); return err
}

func (s *Store) ListMCPAudit(ctx context.Context, classID int64) ([]MCPToolCallAudit, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT a.id, a.server_name, a.tool_name, u.username, a.session_id, a.message_id, a.status, a.error_code, a.created_at FROM mcp_tool_call_audit a JOIN users u ON u.id=a.user_id WHERE a.class_id=? ORDER BY a.id DESC LIMIT 100`, classID)
	if err != nil { return nil, err }; defer rows.Close(); items := []MCPToolCallAudit{}
	for rows.Next() { var item MCPToolCallAudit; var code sql.NullString; if err := rows.Scan(&item.ID, &item.ServerName, &item.ToolName, &item.Username, &item.SessionID, &item.MessageID, &item.Status, &code, &item.CreatedAt); err != nil { return nil, err }; if code.Valid { item.ErrorCode = code.String }; items = append(items, item) }
	return items, rows.Err()
}
