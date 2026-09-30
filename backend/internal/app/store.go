package app

import (
	"campusclaw/backend/migrations"
	"context"
	"crypto/sha256"
	"database/sql"
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
