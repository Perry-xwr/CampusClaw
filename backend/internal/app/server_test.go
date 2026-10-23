package app

import (
	"context"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestRenderPromptTemplateUsesServerContext(t *testing.T) {
	user := User{Username: "demo_student", ClassName: "A 班"}
	got := renderPromptTemplate("班级={{class_name}} 用户={{username}} 问题={{question}}", user, "实验准备？")
	want := "班级=A 班 用户=demo_student 问题=实验准备？"
	if got != want { t.Fatalf("renderPromptTemplate() = %q, want %q", got, want) }
}

func TestWithTestScenarioRequiresOptInAndSafeLabel(t *testing.T) {
	r := httptest.NewRequest("POST", "/", nil)
	r.Header.Set("X-CampusClaw-Test-Scenario", "exp1-prompt-template")
	if _, ok := withTestScenario(context.Background(), r, false).Value(testScenarioContextKey).(string); ok { t.Fatal("scenario accepted while tracing is disabled") }
	if got, ok := withTestScenario(context.Background(), r, true).Value(testScenarioContextKey).(string); !ok || got != "exp1-prompt-template" { t.Fatalf("safe scenario missing: %q", got) }
	r.Header.Set("X-CampusClaw-Test-Scenario", "secret token value")
	if _, ok := withTestScenario(context.Background(), r, true).Value(testScenarioContextKey).(string); ok { t.Fatal("unsafe scenario accepted") }
}

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

func TestBearerToken(t *testing.T) {
	tests := []struct {
		header string
		token  string
		ok     bool
	}{
		{"Bearer session-token", "session-token", true},
		{"", "", false},
		{"Basic session-token", "", false},
		{"Bearer", "", false},
		{"Bearer ", "", false},
		{"Bearer  session-token", "", false},
		{"bearer session-token", "", false},
	}
	for _, test := range tests {
		t.Run(test.header, func(t *testing.T) {
			token, ok := bearerToken(test.header)
			if token != test.token || ok != test.ok {
				t.Fatalf("bearerToken(%q)=(%q,%v), want (%q,%v)", test.header, token, ok, test.token, test.ok)
			}
		})
	}
}
