ALTER TABLE teacher_skills ADD COLUMN tools JSON NULL;
ALTER TABLE chat_messages ADD COLUMN tool_calls JSON NULL;

CREATE TABLE IF NOT EXISTS mcp_servers (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  class_id BIGINT NOT NULL,
  created_by BIGINT NOT NULL,
  name VARCHAR(80) NOT NULL,
  base_url VARCHAR(255) NOT NULL,
  is_connected BOOLEAN NOT NULL DEFAULT FALSE,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  CONSTRAINT fk_mcp_servers_class FOREIGN KEY (class_id) REFERENCES classes(id),
  CONSTRAINT fk_mcp_servers_creator FOREIGN KEY (created_by) REFERENCES users(id),
  UNIQUE KEY uq_mcp_servers_class_name (class_id, name),
  INDEX idx_mcp_servers_class_state (class_id, is_connected)
);

CREATE TABLE IF NOT EXISTS mcp_server_tools (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  server_id BIGINT NOT NULL,
  name VARCHAR(120) NOT NULL,
  description VARCHAR(1000) NOT NULL DEFAULT '',
  input_schema JSON NOT NULL,
  discovered_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  CONSTRAINT fk_mcp_tools_server FOREIGN KEY (server_id) REFERENCES mcp_servers(id) ON DELETE CASCADE,
  UNIQUE KEY uq_mcp_server_tool (server_id, name)
);

CREATE TABLE IF NOT EXISTS mcp_tool_call_audit (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  class_id BIGINT NOT NULL,
  server_id BIGINT NULL,
  server_name VARCHAR(80) NOT NULL,
  tool_name VARCHAR(120) NOT NULL,
  user_id BIGINT NOT NULL,
  session_id BIGINT NOT NULL,
  message_id BIGINT NULL,
  status VARCHAR(16) NOT NULL,
  error_code VARCHAR(64) NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  CONSTRAINT fk_mcp_audit_class FOREIGN KEY (class_id) REFERENCES classes(id),
  CONSTRAINT fk_mcp_audit_user FOREIGN KEY (user_id) REFERENCES users(id),
  INDEX idx_mcp_audit_class_time (class_id, created_at),
  INDEX idx_mcp_audit_session (session_id, created_at)
);
