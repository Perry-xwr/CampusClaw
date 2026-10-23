package app

import (
	"encoding/json"
	"time"
)

type User struct {
	ID        int64  `json:"id"`
	Username  string `json:"username"`
	Role      string `json:"role"`
	ClassID   int64  `json:"class_id"`
	ClassName string `json:"class_name"`
}

type Material struct {
	ID               int64     `json:"id"`
	Title            string    `json:"title"`
	OriginalFilename string    `json:"original_filename"`
	Uploader         string    `json:"uploader"`
	CreatedAt        time.Time `json:"created_at"`
	DeletionPending  bool      `json:"deletion_pending,omitempty"`
}

type Prompt struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Content   string    `json:"content"`
	IsActive  bool      `json:"is_active"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Skill struct {
	ID           int64     `json:"id"`
	Slug         string    `json:"slug"`
	Name         string    `json:"name"`
	Instructions string    `json:"instructions"`
	Tools        []MCPToolBinding `json:"tools"`
	IsActive     bool      `json:"is_active"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type MCPToolBinding struct {
	Ref string `json:"ref"`
}

type MCPServer struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	BaseURL   string    `json:"base_url"`
	Connected bool      `json:"connected"`
	HasToken  bool      `json:"has_token"`
	CreatedAt time.Time `json:"created_at"`
}

type MCPDiscoveredTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"input_schema"`
	DiscoveredAt time.Time      `json:"discovered_at"`
}

type MCPToolCallAudit struct {
	ID         int64     `json:"id"`
	ServerName string    `json:"server_name"`
	ToolName   string    `json:"tool_name"`
	Username   string    `json:"username"`
	SessionID  int64     `json:"session_id"`
	MessageID  *int64    `json:"message_id,omitempty"`
	Status     string    `json:"status"`
	ErrorCode  string    `json:"error_code,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

type MCPToolCallStatus struct {
	Ref    string `json:"ref"`
	Status string `json:"status"`
}

type ChatSession struct {
	ID        int64     `json:"id"`
	Title     string    `json:"title"`
	PromptID  *int64    `json:"prompt_id,omitempty"`
	SkillID   *int64    `json:"skill_id,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type ChatMessage struct {
	ID        int64      `json:"id"`
	Role      string     `json:"role"`
	Content   string     `json:"content"`
	Citations []SearchHit `json:"citations"`
	ToolCalls []MCPToolCallStatus `json:"tool_calls,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}
