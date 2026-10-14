package app

import "time"

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
	IsActive     bool      `json:"is_active"`
	UpdatedAt    time.Time `json:"updated_at"`
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
	CreatedAt time.Time  `json:"created_at"`
}
