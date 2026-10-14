CREATE TABLE IF NOT EXISTS teacher_prompts (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  class_id BIGINT NOT NULL,
  created_by BIGINT NULL,
  name VARCHAR(120) NOT NULL,
  content MEDIUMTEXT NOT NULL,
  is_active BOOLEAN NOT NULL DEFAULT TRUE,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  CONSTRAINT fk_prompts_class FOREIGN KEY (class_id) REFERENCES classes(id),
  CONSTRAINT fk_prompts_creator FOREIGN KEY (created_by) REFERENCES users(id),
  UNIQUE KEY uq_prompts_class_name (class_id, name),
  INDEX idx_prompts_class_active (class_id, is_active)
);

CREATE TABLE IF NOT EXISTS teacher_skills (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  class_id BIGINT NOT NULL,
  created_by BIGINT NULL,
  slug VARCHAR(80) NOT NULL,
  name VARCHAR(120) NOT NULL,
  instructions MEDIUMTEXT NOT NULL,
  is_active BOOLEAN NOT NULL DEFAULT TRUE,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  CONSTRAINT fk_skills_class FOREIGN KEY (class_id) REFERENCES classes(id),
  CONSTRAINT fk_skills_creator FOREIGN KEY (created_by) REFERENCES users(id),
  UNIQUE KEY uq_skills_class_slug (class_id, slug),
  INDEX idx_skills_class_active (class_id, is_active)
);

CREATE TABLE IF NOT EXISTS chat_sessions (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  class_id BIGINT NOT NULL,
  user_id BIGINT NOT NULL,
  prompt_id BIGINT NULL,
  skill_id BIGINT NULL,
  title VARCHAR(200) NOT NULL DEFAULT '新对话',
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  CONSTRAINT fk_chat_sessions_class FOREIGN KEY (class_id) REFERENCES classes(id),
  CONSTRAINT fk_chat_sessions_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
  CONSTRAINT fk_chat_sessions_prompt FOREIGN KEY (prompt_id) REFERENCES teacher_prompts(id) ON DELETE SET NULL,
  CONSTRAINT fk_chat_sessions_skill FOREIGN KEY (skill_id) REFERENCES teacher_skills(id) ON DELETE SET NULL,
  INDEX idx_chat_sessions_user_updated (user_id, updated_at),
  INDEX idx_chat_sessions_class (class_id)
);

CREATE TABLE IF NOT EXISTS chat_messages (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  session_id BIGINT NOT NULL,
  class_id BIGINT NOT NULL,
  role ENUM('user', 'assistant') NOT NULL,
  content MEDIUMTEXT NOT NULL,
  citations JSON NOT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  CONSTRAINT fk_chat_messages_session FOREIGN KEY (session_id) REFERENCES chat_sessions(id) ON DELETE CASCADE,
  CONSTRAINT fk_chat_messages_class FOREIGN KEY (class_id) REFERENCES classes(id),
  INDEX idx_chat_messages_session_created (session_id, created_at, id),
  INDEX idx_chat_messages_class (class_id)
);
