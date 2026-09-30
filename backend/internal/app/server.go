package app

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

type contextKey string

const userContextKey contextKey = "user"

type Server struct {
	config Config
	store  *Store
	mux    *http.ServeMux
	upload sync.Mutex
}

func NewServer(config Config, store *Store) *Server {
	s := &Server{config: config, store: store, mux: http.NewServeMux()}
	s.routes()
	return s
}

func (s *Server) Handler() http.Handler { return recoverMiddleware(securityHeaders(s.mux)) }

func (s *Server) routes() {
	s.mux.HandleFunc("GET /health", s.health)
	s.mux.HandleFunc("POST /api/login", s.login)
	s.mux.Handle("POST /api/logout", s.auth(http.HandlerFunc(s.logout)))
	s.mux.Handle("GET /api/me", s.auth(http.HandlerFunc(s.me)))
	s.mux.Handle("GET /api/materials", s.auth(http.HandlerFunc(s.listMaterials)))
	s.mux.Handle("GET /api/materials/{id}", s.auth(http.HandlerFunc(s.getMaterial)))
	s.mux.Handle("GET /api/materials/{id}/file", s.auth(http.HandlerFunc(s.getMaterialFile)))
	s.mux.Handle("POST /api/materials", s.auth(s.teacher(http.HandlerFunc(s.createMaterial))))
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := s.store.DB.PingContext(ctx); err != nil {
		writeError(w, http.StatusServiceUnavailable, "database unavailable")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	var request struct{ Username, Password string }
	if err := decodeJSON(r, &request); err != nil || request.Username == "" || request.Password == "" {
		writeError(w, http.StatusBadRequest, "username and password are required")
		return
	}
	user, err := s.store.Authenticate(r.Context(), request.Username, request.Password)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid username or password")
		return
	}
	token, err := randomToken(32)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not create session")
		return
	}
	expires := time.Now().Add(s.config.SessionTTL)
	if err := s.store.CreateSession(r.Context(), user.ID, token, expires); err != nil {
		writeError(w, http.StatusInternalServerError, "could not create session")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"token": token, "user": user})
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	token, _ := bearerToken(r.Header.Get("Authorization"))
	_ = s.store.DeleteSession(r.Context(), token)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, currentUser(r.Context()))
}

func (s *Server) listMaterials(w http.ResponseWriter, r *http.Request) {
	items, err := s.store.Materials(r.Context(), currentUser(r.Context()).ClassID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not list materials")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"materials": items})
}

func materialID(r *http.Request) (int64, error) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id < 1 {
		return 0, ErrNotFound
	}
	return id, nil
}

func (s *Server) getMaterial(w http.ResponseWriter, r *http.Request) {
	id, err := materialID(r)
	if err != nil {
		writeError(w, http.StatusNotFound, "material not found")
		return
	}
	item, _, err := s.store.Material(r.Context(), id, currentUser(r.Context()).ClassID)
	if err != nil {
		writeError(w, http.StatusNotFound, "material not found")
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) getMaterialFile(w http.ResponseWriter, r *http.Request) {
	id, err := materialID(r)
	if err != nil {
		writeError(w, http.StatusNotFound, "material not found")
		return
	}
	item, stored, err := s.store.Material(r.Context(), id, currentUser(r.Context()).ClassID)
	if err != nil {
		writeError(w, http.StatusNotFound, "material not found")
		return
	}
	path := filepath.Join(s.config.UploadDir, filepath.Base(stored))
	if _, err := os.Stat(path); err != nil {
		writeError(w, http.StatusNotFound, "material not found")
		return
	}
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename*=UTF-8''%s", encodeFilename(item.OriginalFilename)))
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	http.ServeFile(w, r, path)
}

func (s *Server) createMaterial(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, s.config.MaxUploadBytes+64*1024)
	if err := r.ParseMultipartForm(s.config.MaxUploadBytes + 32*1024); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			writeError(w, http.StatusRequestEntityTooLarge, "upload exceeds size limit")
		} else {
			writeError(w, http.StatusBadRequest, "invalid multipart request")
		}
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "file is required")
		return
	}
	defer file.Close()
	content, status, message := readValidatedUpload(file, header, s.config.MaxUploadBytes)
	if status != 0 {
		writeError(w, status, message)
		return
	}
	title := strings.TrimSpace(r.FormValue("title"))
	if title == "" {
		title = strings.TrimSuffix(filepath.Base(header.Filename), filepath.Ext(header.Filename))
	}
	if title == "" || utf8.RuneCountInString(title) > 200 {
		writeError(w, http.StatusBadRequest, "title must be 1 to 200 characters")
		return
	}
	user := currentUser(r.Context())
	storedToken, err := randomToken(18)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not store upload")
		return
	}
	storedName := storedToken + strings.ToLower(filepath.Ext(header.Filename))

	s.upload.Lock()
	defer s.upload.Unlock()
	if err := os.MkdirAll(s.config.UploadDir, 0750); err != nil {
		writeError(w, http.StatusInternalServerError, "could not store upload")
		return
	}
	temp, err := os.CreateTemp(s.config.UploadDir, ".incoming-*")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not store upload")
		return
	}
	tempName, finalName := temp.Name(), filepath.Join(s.config.UploadDir, storedName)
	cleanup := func() { _ = os.Remove(tempName); _ = os.Remove(finalName) }
	if _, err := temp.Write(content); err != nil {
		_ = temp.Close()
		cleanup()
		writeError(w, http.StatusInternalServerError, "could not store upload")
		return
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		cleanup()
		writeError(w, http.StatusInternalServerError, "could not store upload")
		return
	}
	if err := temp.Close(); err != nil {
		cleanup()
		writeError(w, http.StatusInternalServerError, "could not store upload")
		return
	}
	tx, err := s.store.DB.BeginTx(r.Context(), nil)
	if err != nil {
		cleanup()
		writeError(w, http.StatusInternalServerError, "could not store upload")
		return
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(r.Context(), `INSERT INTO materials (class_id, uploader_id, title, original_filename, stored_filename) VALUES (?, ?, ?, ?, ?)`, user.ClassID, user.ID, title, filepath.Base(header.Filename), storedName)
	if err != nil {
		cleanup()
		writeError(w, http.StatusInternalServerError, "could not store upload")
		return
	}
	id, _ := result.LastInsertId()
	if _, err := tx.ExecContext(r.Context(), `INSERT INTO knowledge_entries (material_id, class_id, content) VALUES (?, ?, ?)`, id, user.ClassID, string(content)); err != nil {
		cleanup()
		writeError(w, http.StatusInternalServerError, "could not ingest upload")
		return
	}
	if err := os.Rename(tempName, finalName); err != nil {
		cleanup()
		writeError(w, http.StatusInternalServerError, "could not store upload")
		return
	}
	if err := tx.Commit(); err != nil {
		cleanup()
		writeError(w, http.StatusInternalServerError, "could not commit upload")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": id, "title": title})
}

func readValidatedUpload(file multipart.File, header *multipart.FileHeader, limit int64) ([]byte, int, string) {
	ext := strings.ToLower(filepath.Ext(header.Filename))
	if ext != ".txt" && ext != ".md" {
		return nil, http.StatusBadRequest, "only .txt and .md files are supported"
	}
	content, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return nil, http.StatusBadRequest, "could not read upload"
	}
	if int64(len(content)) > limit {
		return nil, http.StatusRequestEntityTooLarge, "upload exceeds size limit"
	}
	if len(content) == 0 {
		return nil, http.StatusBadRequest, "file must not be empty"
	}
	if !utf8.Valid(content) {
		return nil, http.StatusBadRequest, "file must be valid UTF-8"
	}
	return content, 0, ""
}

func (s *Server) auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, ok := bearerToken(r.Header.Get("Authorization"))
		if !ok {
			writeError(w, http.StatusUnauthorized, "authentication required")
			return
		}
		user, err := s.store.UserBySession(r.Context(), token)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "authentication required")
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userContextKey, user)))
	})
}

func bearerToken(header string) (string, bool) {
	parts := strings.Split(header, " ")
	if len(parts) != 2 || parts[0] != "Bearer" || parts[1] == "" || strings.TrimSpace(parts[1]) != parts[1] {
		return "", false
	}
	return parts[1], true
}

func (s *Server) teacher(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if currentUser(r.Context()).Role != "teacher" {
			writeError(w, http.StatusForbidden, "teacher role required")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func currentUser(ctx context.Context) User { return ctx.Value(userContextKey).(User) }

func randomToken(size int) (string, error) {
	buffer := make([]byte, size)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buffer), nil
}

func decodeJSON(r *http.Request, target any) error {
	decoder := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	return decoder.Decode(target)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "same-origin")
		next.ServeHTTP(w, r)
	})
}

func recoverMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recover() != nil {
				writeError(w, http.StatusInternalServerError, "internal server error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func encodeFilename(name string) string {
	var builder strings.Builder
	for _, b := range []byte(filepath.Base(name)) {
		if (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9') || strings.ContainsRune("-._~", rune(b)) {
			builder.WriteByte(b)
		} else {
			builder.WriteString(fmt.Sprintf("%%%02X", b))
		}
	}
	return builder.String()
}
