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
