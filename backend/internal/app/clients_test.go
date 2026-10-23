package app

import (
	"context"
	"encoding/json"
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

func TestQdrantDeleteMaterialUsesClassScopedFilterAndWait(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/collections/campusclaw_chunks/points/delete" || r.URL.Query().Get("wait") != "true" {
			t.Errorf("unexpected delete request: %s %s", r.Method, r.URL.String())
		}
		var body struct {
			Filter struct {
				Must []struct { Key string `json:"key"`; Match struct { Value int64 `json:"value"` } `json:"match"` } `json:"must"`
			} `json:"filter"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil { t.Errorf("decode body: %v", err); return }
		if len(body.Filter.Must) != 2 || body.Filter.Must[0].Key != "material_id" || body.Filter.Must[0].Match.Value != 81 || body.Filter.Must[1].Key != "class_id" || body.Filter.Must[1].Match.Value != 4 {
			t.Errorf("delete filter was not scoped to material and class: %+v", body.Filter.Must)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"result":{"status":"completed"}}`))
	}))
	defer server.Close()
	client := &QdrantClient{baseURL: server.URL, collection: "campusclaw_chunks", http: &http.Client{Timeout: time.Second}}
	if err := client.DeleteMaterialPoints(context.Background(), 81, 4); err != nil { t.Fatal(err) }
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

func TestChatStreamUsesContentAndIgnoresReasoningContent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-chat-key" {
			t.Errorf("missing chat authorization header")
		}
		if r.URL.Path != "/v4/chat/completions" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"reasoning_content\":\"private reasoning\"}}]}\n\n")
		_, _ = fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"final answer\"}}]}\n\n")
		_, _ = fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer server.Close()

	config := modelTestConfig(server.URL + "/v4")
	config.ChatAPIKey = "test-chat-key"
	client := NewModelClient(config)
	var streamed strings.Builder
	answer, err := client.ChatStream(context.Background(), "system", []ChatMessage{{Role: "user", Content: "question"}}, func(token string) error {
		streamed.WriteString(token)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if answer != "final answer" || streamed.String() != answer || strings.Contains(answer, "reasoning") {
		t.Fatalf("unexpected answer=%q streamed=%q", answer, streamed.String())
	}
}

func TestConfigRejectsInvalidDimension(t *testing.T) {
	t.Setenv("EMBEDDING_DIMENSION", "0")
	if _, err := ConfigFromEnv(); err == nil {
		t.Fatal("expected dimension validation error")
	}
}
