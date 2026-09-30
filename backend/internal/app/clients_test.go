package app

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func modelTestConfig(url string) Config {
	return Config{EmbeddingBaseURL: url, EmbeddingAPIKey: "do-not-leak", EmbeddingModel: "test", EmbeddingDimension: 3, ChatBaseURL: url, ChatModel: "test", ModelTimeout: time.Second}
}

func TestEmbeddingClientValidation(t *testing.T) {
	tests := []struct {
		name    string
		status  int
		body    string
		wantErr bool
	}{
		{"success", 200, `{"data":[{"index":0,"embedding":[1,0,0]}]}`, false},
		{"wrong dimension", 200, `{"data":[{"index":0,"embedding":[1,0]}]}`, true},
		{"malformed", 200, `{"data":`, true},
		{"http failure", 503, `{"error":"down"}`, true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Authorization") != "Bearer do-not-leak" {
					t.Error("missing authorization")
				}
				w.WriteHeader(test.status)
				_, _ = w.Write([]byte(test.body))
			}))
			defer server.Close()
			client := NewModelClient(modelTestConfig(server.URL))
			_, err := client.Embed(context.Background(), []string{"hello"})
			if (err != nil) != test.wantErr {
				t.Fatalf("err=%v wantErr=%v", err, test.wantErr)
			}
			if err != nil && strings.Contains(err.Error(), "do-not-leak") {
				t.Fatal("secret leaked in error")
			}
		})
	}
}

func TestQdrantCollectionMismatch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `{"result":{"config":{"params":{"vectors":{"size":99,"distance":"Dot"}}}}}`)
	}))
	defer server.Close()
	client := &QdrantClient{baseURL: server.URL, collection: "campusclaw_chunks", dimension: 16, http: &http.Client{Timeout: time.Second}}
	err := client.EnsureCollection(context.Background())
	if err == nil || !strings.Contains(err.Error(), "collection mismatch") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestChatClientValidation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `{"choices":[{"message":{"content":"grounded [1]"}}]}`)
	}))
	defer server.Close()
	client := NewModelClient(modelTestConfig(server.URL))
	answer, err := client.Chat(context.Background(), "system", "evidence")
	if err != nil || answer != "grounded [1]" {
		t.Fatalf("answer=%q err=%v", answer, err)
	}
}

func TestConfigRejectsInvalidDimension(t *testing.T) {
	t.Setenv("EMBEDDING_DIMENSION", "0")
	if _, err := ConfigFromEnv(); err == nil {
		t.Fatal("expected dimension validation error")
	}
}
