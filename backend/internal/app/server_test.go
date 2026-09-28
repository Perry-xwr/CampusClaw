package app

import (
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

func uploadFile(t *testing.T, name string, content []byte, limit int64) ([]byte, int, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "upload")
	if err := os.WriteFile(path, content, 0600); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	return readValidatedUpload(file, &multipart.FileHeader{Filename: name}, limit)
}

func TestReadValidatedUpload(t *testing.T) {
	tests := []struct {
		name       string
		filename   string
		content    []byte
		limit      int64
		wantStatus int
	}{
		{"markdown", "lesson.md", []byte("# Hello\n"), 1024, 0},
		{"text uppercase extension", "lesson.TXT", []byte("hello"), 1024, 0},
		{"unsupported", "lesson.exe", []byte("hello"), 1024, http.StatusBadRequest},
		{"empty", "lesson.md", nil, 1024, http.StatusBadRequest},
		{"invalid utf8", "lesson.txt", []byte{0xff, 0xfe}, 1024, http.StatusBadRequest},
		{"too large", "lesson.txt", []byte("12345"), 4, http.StatusRequestEntityTooLarge},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			content, status, _ := uploadFile(t, test.filename, test.content, test.limit)
			if status != test.wantStatus {
				t.Fatalf("status=%d want=%d", status, test.wantStatus)
			}
			if status == 0 && string(content) != string(test.content) {
				t.Fatalf("content=%q", content)
			}
		})
	}
}

func TestEncodeFilename(t *testing.T) {
	if got := encodeFilename("../讲义 1.md"); got != "%E8%AE%B2%E4%B9%89%201.md" {
		t.Fatalf("encoded filename=%q", got)
	}
}
