package app

import (
	"campusclaw/backend/migrations"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"golang.org/x/crypto/bcrypt"
)

var ErrNotFound = errors.New("not found")

type Store struct{ DB *sql.DB }

func OpenStore(ctx context.Context, dsn string) (*Store, error) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(15)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(3 * time.Minute)

	var lastErr error
	for attempt := 0; attempt < 30; attempt++ {
		if lastErr = db.PingContext(ctx); lastErr == nil {
			return &Store{DB: db}, nil
		}
		select {
		case <-ctx.Done():
			db.Close()
			return nil, ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
	db.Close()
	return nil, fmt.Errorf("database unavailable: %w", lastErr)
}

func (s *Store) Migrate(ctx context.Context) error {
	entries, err := migrations.Files.ReadDir(".")
	if err != nil {
		return err
	}
	for _, entry := range entries {
		body, err := migrations.Files.ReadFile(entry.Name())
		if err != nil {
			return err
		}
		for _, statement := range strings.Split(string(body), ";") {
			if strings.TrimSpace(statement) == "" {
				continue
			}
			if _, err := s.DB.ExecContext(ctx, statement); err != nil {
				return fmt.Errorf("migration %s: %w", entry.Name(), err)
			}
		}
	}
	return s.seed(ctx)
}

func (s *Store) seed(ctx context.Context) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `INSERT IGNORE INTO classes (id, name) VALUES (1, 'A 班'), (2, 'B 班')`); err != nil {
		return err
	}
	accounts := []struct {
		id, classID              int64
		username, role, password string
	}{
		{1, 1, "teacher_a", "teacher", "TeacherA123!"},
		{2, 1, "student_a1", "student", "StudentA123!"},
		{3, 2, "student_b1", "student", "StudentB123!"},
	}
	for _, account := range accounts {
		var exists int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE username=?`, account.username).Scan(&exists); err != nil {
			return err
		}
		if exists == 0 {
			hash, err := bcrypt.GenerateFromPassword([]byte(account.password), bcrypt.DefaultCost)
			if err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `INSERT INTO users (id, username, password_hash, role, class_id) VALUES (?, ?, ?, ?, ?)`, account.id, account.username, string(hash), account.role, account.classID); err != nil {
				return err
			}
		}
	}
	seedMaterials := []struct {
		id, classID, uploaderID  int64
		title, filename, content string
	}{
		{1, 1, 1, "A 班太阳能实验讲义", "seed-a.txt", "A班课程的太阳能电池实验使用钙钛矿材料。学生需要在实验前完成安全培训，并佩戴护目镜。"},
		{2, 2, 3, "B 班量子通信讲义", "seed-b.txt", "B班独有的量子通信实验使用纠缠光子，并通过量子密钥分发验证通信安全。"},
	}
	for _, item := range seedMaterials {
		if _, err := tx.ExecContext(ctx, `INSERT IGNORE INTO materials (id, class_id, uploader_id, title, original_filename, stored_filename) VALUES (?, ?, ?, ?, ?, ?)`, item.id, item.classID, item.uploaderID, item.title, item.filename, item.filename); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT IGNORE INTO knowledge_entries (material_id, class_id, content) VALUES (?, ?, ?)`, item.id, item.classID, item.content); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE materials SET title=? WHERE id=? AND class_id=?`, item.title, item.id, item.classID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE knowledge_entries SET content=? WHERE material_id=? AND class_id=?`, item.content, item.id, item.classID); err != nil {
			return err
		}
	}
	defaultPrompt := "你是 CampusClaw 班级课程助教。回答必须依据当前会话的本班检索资料，明确区分资料事实与推理；无法从资料确认时应说明。"
	defaultSkill := "通过提问引导学生识别题目条件、关联概念并分步骤思考。每次只提出一个简短提示或问题；不要直接给出最终答案、代算最终结果或替学生完成题目。"
	for _, classID := range []int64{1, 2} {
		if _, err := tx.ExecContext(ctx, `INSERT IGNORE INTO teacher_prompts (class_id, created_by, name, content) VALUES (?, NULL, '课程知识助教', ?)`, classID, defaultPrompt); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT IGNORE INTO teacher_skills (class_id, created_by, slug, name, instructions) VALUES (?, NULL, 'guided-solution', '解题引导', ?)`, classID, defaultSkill); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func tokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func (s *Store) Authenticate(ctx context.Context, username, password string) (User, error) {
	var user User
	var hash string
	err := s.DB.QueryRowContext(ctx, `SELECT u.id, u.username, u.password_hash, u.role, u.class_id, c.name FROM users u JOIN classes c ON c.id=u.class_id WHERE u.username=?`, username).
		Scan(&user.ID, &user.Username, &hash, &user.Role, &user.ClassID, &user.ClassName)
	if err != nil || bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) != nil {
		return User{}, ErrNotFound
	}
	return user, nil
}

func (s *Store) CreateSession(ctx context.Context, userID int64, token string, expires time.Time) error {
	_, err := s.DB.ExecContext(ctx, `INSERT INTO sessions (token_hash, user_id, expires_at) VALUES (?, ?, ?)`, tokenHash(token), userID, expires)
	return err
}

func (s *Store) UserBySession(ctx context.Context, token string) (User, error) {
	var user User
	err := s.DB.QueryRowContext(ctx, `SELECT u.id, u.username, u.role, u.class_id, c.name FROM sessions s JOIN users u ON u.id=s.user_id JOIN classes c ON c.id=u.class_id WHERE s.token_hash=? AND s.expires_at>NOW()`, tokenHash(token)).
		Scan(&user.ID, &user.Username, &user.Role, &user.ClassID, &user.ClassName)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrNotFound
	}
	return user, err
}

func (s *Store) DeleteSession(ctx context.Context, token string) error {
	_, err := s.DB.ExecContext(ctx, `DELETE FROM sessions WHERE token_hash=?`, tokenHash(token))
	return err
}

func (s *Store) Materials(ctx context.Context, classID int64) ([]Material, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT m.id, m.title, m.original_filename, u.username, m.created_at FROM materials m JOIN users u ON u.id=m.uploader_id WHERE m.class_id=? ORDER BY m.created_at DESC, m.id DESC`, classID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Material{}
	for rows.Next() {
		var item Material
		if err := rows.Scan(&item.ID, &item.Title, &item.OriginalFilename, &item.Uploader, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) Material(ctx context.Context, id, classID int64) (Material, string, error) {
	var item Material
	var stored string
	err := s.DB.QueryRowContext(ctx, `SELECT m.id, m.title, m.original_filename, u.username, m.created_at, m.stored_filename FROM materials m JOIN users u ON u.id=m.uploader_id WHERE m.id=? AND m.class_id=?`, id, classID).
		Scan(&item.ID, &item.Title, &item.OriginalFilename, &item.Uploader, &item.CreatedAt, &stored)
	if errors.Is(err, sql.ErrNoRows) {
		return Material{}, "", ErrNotFound
	}
	return item, stored, err
}

func (s *Store) ListPrompts(ctx context.Context, classID int64) ([]Prompt, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT id, name, content, is_active, updated_at FROM teacher_prompts WHERE class_id=? ORDER BY is_active DESC, id`, classID)
	if err != nil { return nil, err }
	defer rows.Close()
	items := []Prompt{}
	for rows.Next() { var item Prompt; if err := rows.Scan(&item.ID, &item.Name, &item.Content, &item.IsActive, &item.UpdatedAt); err != nil { return nil, err }; items = append(items, item) }
	return items, rows.Err()
}

func (s *Store) SavePrompt(ctx context.Context, classID, userID, id int64, name, content string, active bool) (Prompt, error) {
	if id == 0 {
		result, err := s.DB.ExecContext(ctx, `INSERT INTO teacher_prompts (class_id, created_by, name, content, is_active) VALUES (?, ?, ?, ?, ?)`, classID, userID, name, content, active)
		if err != nil { return Prompt{}, err }
		id, err = result.LastInsertId(); if err != nil { return Prompt{}, err }
	} else {
		result, err := s.DB.ExecContext(ctx, `UPDATE teacher_prompts SET name=?, content=?, is_active=? WHERE id=? AND class_id=?`, name, content, active, id, classID)
		if err != nil { return Prompt{}, err }
		changed, err := result.RowsAffected(); if err != nil { return Prompt{}, err }; if changed == 0 { var exists int; if err := s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM teacher_prompts WHERE id=? AND class_id=?`, id, classID).Scan(&exists); err != nil { return Prompt{}, err }; if exists == 0 { return Prompt{}, ErrNotFound } }
	}
	var item Prompt
	err := s.DB.QueryRowContext(ctx, `SELECT id, name, content, is_active, updated_at FROM teacher_prompts WHERE id=? AND class_id=?`, id, classID).Scan(&item.ID, &item.Name, &item.Content, &item.IsActive, &item.UpdatedAt)
	return item, err
}

func (s *Store) ListSkills(ctx context.Context, classID int64, activeOnly bool) ([]Skill, error) {
	query := `SELECT id, slug, name, instructions, is_active, updated_at FROM teacher_skills WHERE class_id=?`
	if activeOnly { query += ` AND is_active=TRUE` }
	query += ` ORDER BY is_active DESC, id`
	rows, err := s.DB.QueryContext(ctx, query, classID); if err != nil { return nil, err }; defer rows.Close()
	items := []Skill{}
	for rows.Next() { var item Skill; if err := rows.Scan(&item.ID, &item.Slug, &item.Name, &item.Instructions, &item.IsActive, &item.UpdatedAt); err != nil { return nil, err }; items = append(items, item) }
	return items, rows.Err()
}

func (s *Store) SaveSkill(ctx context.Context, classID, userID, id int64, slug, name, instructions string, active bool) (Skill, error) {
	if id == 0 {
		result, err := s.DB.ExecContext(ctx, `INSERT INTO teacher_skills (class_id, created_by, slug, name, instructions, is_active) VALUES (?, ?, ?, ?, ?, ?)`, classID, userID, slug, name, instructions, active)
		if err != nil { return Skill{}, err }; id, err = result.LastInsertId(); if err != nil { return Skill{}, err }
	} else {
		result, err := s.DB.ExecContext(ctx, `UPDATE teacher_skills SET slug=?, name=?, instructions=?, is_active=? WHERE id=? AND class_id=?`, slug, name, instructions, active, id, classID)
		if err != nil { return Skill{}, err }; changed, err := result.RowsAffected(); if err != nil { return Skill{}, err }; if changed == 0 { var exists int; if err := s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM teacher_skills WHERE id=? AND class_id=?`, id, classID).Scan(&exists); err != nil { return Skill{}, err }; if exists == 0 { return Skill{}, ErrNotFound } }
	}
	var item Skill
	err := s.DB.QueryRowContext(ctx, `SELECT id, slug, name, instructions, is_active, updated_at FROM teacher_skills WHERE id=? AND class_id=?`, id, classID).Scan(&item.ID, &item.Slug, &item.Name, &item.Instructions, &item.IsActive, &item.UpdatedAt)
	return item, err
}

func (s *Store) CreateChatSession(ctx context.Context, classID, userID int64, promptID, skillID *int64, title string) (ChatSession, error) {
	result, err := s.DB.ExecContext(ctx, `INSERT INTO chat_sessions (class_id, user_id, prompt_id, skill_id, title) VALUES (?, ?, ?, ?, ?)`, classID, userID, promptID, skillID, title)
	if err != nil { return ChatSession{}, err }; id, err := result.LastInsertId(); if err != nil { return ChatSession{}, err }
	return s.ChatSession(ctx, id, userID, classID)
}

func (s *Store) ChatSession(ctx context.Context, id, userID, classID int64) (ChatSession, error) {
	var item ChatSession
	err := s.DB.QueryRowContext(ctx, `SELECT id, title, prompt_id, skill_id, created_at, updated_at FROM chat_sessions WHERE id=? AND user_id=? AND class_id=?`, id, userID, classID).Scan(&item.ID, &item.Title, &item.PromptID, &item.SkillID, &item.CreatedAt, &item.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) { return ChatSession{}, ErrNotFound }; return item, err
}

func (s *Store) ListChatSessions(ctx context.Context, userID, classID int64) ([]ChatSession, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT id, title, prompt_id, skill_id, created_at, updated_at FROM chat_sessions WHERE user_id=? AND class_id=? ORDER BY updated_at DESC, id DESC`, userID, classID)
	if err != nil { return nil, err }; defer rows.Close(); items := []ChatSession{}
	for rows.Next() { var item ChatSession; if err := rows.Scan(&item.ID, &item.Title, &item.PromptID, &item.SkillID, &item.CreatedAt, &item.UpdatedAt); err != nil { return nil, err }; items = append(items, item) }; return items, rows.Err()
}

func (s *Store) PromptAndSkill(ctx context.Context, promptID, skillID *int64, classID int64, requireActive bool) (Prompt, Skill, error) {
	var prompt Prompt; var skill Skill
	if promptID != nil {
		query := `SELECT id, name, content FROM teacher_prompts WHERE id=? AND class_id=?`; if requireActive { query += ` AND is_active=TRUE` }
		err := s.DB.QueryRowContext(ctx, query, *promptID, classID).Scan(&prompt.ID, &prompt.Name, &prompt.Content)
		if err != nil { return Prompt{}, Skill{}, ErrNotFound }
	} else if err := s.DB.QueryRowContext(ctx, `SELECT id, name, content FROM teacher_prompts WHERE class_id=? AND is_active=TRUE ORDER BY id LIMIT 1`, classID).Scan(&prompt.ID, &prompt.Name, &prompt.Content); err != nil { return Prompt{}, Skill{}, err }
	if skillID != nil {
		query := `SELECT id, slug, name, instructions FROM teacher_skills WHERE id=? AND class_id=?`; if requireActive { query += ` AND is_active=TRUE` }
		err := s.DB.QueryRowContext(ctx, query, *skillID, classID).Scan(&skill.ID, &skill.Slug, &skill.Name, &skill.Instructions)
		if err != nil { return Prompt{}, Skill{}, ErrNotFound }
	} else if err := s.DB.QueryRowContext(ctx, `SELECT id, slug, name, instructions FROM teacher_skills WHERE class_id=? AND is_active=TRUE ORDER BY id LIMIT 1`, classID).Scan(&skill.ID, &skill.Slug, &skill.Name, &skill.Instructions); err != nil { return Prompt{}, Skill{}, err }
	return prompt, skill, nil
}

func (s *Store) AddChatMessage(ctx context.Context, sessionID, classID int64, role, content string, citations []SearchHit) (int64, error) {
	encoded, err := json.Marshal(citations); if err != nil { return 0, err }
	result, err := s.DB.ExecContext(ctx, `INSERT INTO chat_messages (session_id, class_id, role, content, citations) VALUES (?, ?, ?, ?, ?)`, sessionID, classID, role, content, string(encoded))
	if err != nil { return 0, err }; return result.LastInsertId()
}

func (s *Store) ChatHistory(ctx context.Context, sessionID int64, limit int) ([]ChatMessage, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT id, role, content, citations, created_at FROM (SELECT id, role, content, citations, created_at FROM chat_messages WHERE session_id=? ORDER BY id DESC LIMIT ?) recent ORDER BY id`, sessionID, limit)
	if err != nil { return nil, err }; defer rows.Close(); items := []ChatMessage{}
	for rows.Next() { var item ChatMessage; var encoded []byte; if err := rows.Scan(&item.ID, &item.Role, &item.Content, &encoded, &item.CreatedAt); err != nil { return nil, err }; if err := json.Unmarshal(encoded, &item.Citations); err != nil { return nil, err }; items = append(items, item) }; return items, rows.Err()
}
