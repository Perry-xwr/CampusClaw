CREATE TABLE IF NOT EXISTS knowledge_chunks (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  knowledge_entry_id BIGINT NOT NULL,
  material_id BIGINT NOT NULL,
  class_id BIGINT NOT NULL,
  chunk_index INT NOT NULL,
  chunk_text MEDIUMTEXT NOT NULL,
  start_offset INT NOT NULL,
  end_offset INT NOT NULL,
  index_status ENUM('pending', 'ready', 'failed') NOT NULL DEFAULT 'pending',
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  CONSTRAINT fk_chunks_entry FOREIGN KEY (knowledge_entry_id) REFERENCES knowledge_entries(id) ON DELETE CASCADE,
  CONSTRAINT fk_chunks_material FOREIGN KEY (material_id) REFERENCES materials(id) ON DELETE CASCADE,
  CONSTRAINT fk_chunks_class FOREIGN KEY (class_id) REFERENCES classes(id),
  UNIQUE KEY uq_chunks_entry_index (knowledge_entry_id, chunk_index),
  INDEX idx_chunks_class_status (class_id, index_status),
  INDEX idx_chunks_material (material_id),
  FULLTEXT KEY ft_chunks_text (chunk_text) WITH PARSER ngram
);
