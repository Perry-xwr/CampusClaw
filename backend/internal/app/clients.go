package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"bufio"
	"strings"
	"time"
)

var ErrDependencyUnavailable = errors.New("retrieval dependency unavailable")

type ModelClient struct {
	embeddingURL   string
	embeddingKey   string
	embeddingModel string
	chatURL        string
	chatKey        string
	chatModel      string
	dimension      int
	http           *http.Client
}

func NewModelClient(config Config) *ModelClient {
	return &ModelClient{
		embeddingURL:   strings.TrimRight(config.EmbeddingBaseURL, "/") + "/embeddings",
		embeddingKey:   config.EmbeddingAPIKey,
		embeddingModel: config.EmbeddingModel,
		chatURL:        strings.TrimRight(config.ChatBaseURL, "/") + "/chat/completions",
		chatKey:        config.ChatAPIKey,
		chatModel:      config.ChatModel,
		dimension:      config.EmbeddingDimension,
		http:           &http.Client{Timeout: config.ModelTimeout},
	}
}

func (client *ModelClient) Embed(ctx context.Context, inputs []string) ([][]float32, error) {
	request := struct {
		Model string   `json:"model"`
		Input []string `json:"input"`
	}{client.embeddingModel, inputs}
	var response struct {
		Data []struct {
			Index     int       `json:"index"`
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
	}
	if err := client.post(ctx, client.embeddingURL, client.embeddingKey, request, &response); err != nil {
		return nil, fmt.Errorf("embedding request failed: %w", ErrDependencyUnavailable)
	}
	if len(response.Data) != len(inputs) {
		return nil, fmt.Errorf("embedding response count mismatch")
	}
	result := make([][]float32, len(inputs))
	for _, item := range response.Data {
		if item.Index < 0 || item.Index >= len(result) || len(item.Embedding) != client.dimension {
			return nil, fmt.Errorf("embedding response dimension mismatch: want %d", client.dimension)
		}
		result[item.Index] = item.Embedding
	}
	for _, vector := range result {
		if len(vector) != client.dimension {
			return nil, fmt.Errorf("embedding response is incomplete")
		}
	}
	return result, nil
}

func (client *ModelClient) Chat(ctx context.Context, system, prompt string) (string, error) {
	request := map[string]any{
		"model":    client.chatModel,
		"stream":   false,
		"messages": []map[string]string{{"role": "system", "content": system}, {"role": "user", "content": prompt}},
	}
	var response struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := client.post(ctx, client.chatURL, client.chatKey, request, &response); err != nil || len(response.Choices) == 0 || strings.TrimSpace(response.Choices[0].Message.Content) == "" {
		return "", fmt.Errorf("chat request failed: %w", ErrDependencyUnavailable)
	}
	return response.Choices[0].Message.Content, nil
}

func (client *ModelClient) ChatStream(ctx context.Context, system string, history []ChatMessage, onToken func(string) error) (string, error) {
	messages := []map[string]string{{"role": "system", "content": system}}
	for _, message := range history {
		if message.Role == "user" || message.Role == "assistant" {
			messages = append(messages, map[string]string{"role": message.Role, "content": message.Content})
		}
	}
	body, err := json.Marshal(map[string]any{"model": client.chatModel, "stream": true, "messages": messages})
	if err != nil { return "", err }
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, client.chatURL, bytes.NewReader(body))
	if err != nil { return "", err }
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "text/event-stream")
	if client.chatKey != "" { request.Header.Set("Authorization", "Bearer "+client.chatKey) }
	response, err := client.http.Do(request)
	if err != nil { return "", fmt.Errorf("chat stream failed: %w", ErrDependencyUnavailable) }
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 { _, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096)); return "", fmt.Errorf("chat stream HTTP %d: %w", response.StatusCode, ErrDependencyUnavailable) }
	reader := bufio.NewReader(io.LimitReader(response.Body, 4<<20))
	var answer strings.Builder
	for {
		line, readErr := reader.ReadString('\n')
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "data:") {
			data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			if data == "[DONE]" { break }
			var event struct { Choices []struct { Delta struct { Content string `json:"content"` } `json:"delta"` } `json:"choices"` }
			if err := json.Unmarshal([]byte(data), &event); err != nil { return answer.String(), fmt.Errorf("invalid chat stream: %w", ErrDependencyUnavailable) }
			if len(event.Choices) > 0 && event.Choices[0].Delta.Content != "" {
				token := event.Choices[0].Delta.Content
				if err := onToken(token); err != nil { return answer.String(), err }
				answer.WriteString(token)
			}
		}
		if readErr != nil { if readErr != io.EOF { return answer.String(), fmt.Errorf("read chat stream: %w", ErrDependencyUnavailable) }; break }
	}
	if strings.TrimSpace(answer.String()) == "" { return "", fmt.Errorf("empty chat stream: %w", ErrDependencyUnavailable) }
	return answer.String(), nil
}

func (client *ModelClient) post(ctx context.Context, url, key string, requestBody, responseBody any) error {
	body, err := json.Marshal(requestBody)
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	if key != "" {
		request.Header.Set("Authorization", "Bearer "+key)
	}
	response, err := client.http.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
		return fmt.Errorf("upstream HTTP %d", response.StatusCode)
	}
	decoder := json.NewDecoder(io.LimitReader(response.Body, 4<<20))
	return decoder.Decode(responseBody)
}

type QdrantClient struct {
	baseURL    string
	collection string
	dimension  int
	http       *http.Client
}

type VectorPoint struct {
	ID               int64
	Vector           []float32
	ClassID          int64
	MaterialID       int64
	KnowledgeEntryID int64
	ChunkIndex       int
}

type VectorMatch struct {
	ID    int64
	Score float64
}

func NewQdrantClient(config Config) *QdrantClient {
	return &QdrantClient{baseURL: strings.TrimRight(config.QdrantURL, "/"), collection: config.QdrantCollection, dimension: config.EmbeddingDimension, http: &http.Client{Timeout: 8 * time.Second}}
}

func (client *QdrantClient) EnsureCollection(ctx context.Context) error {
	url := client.baseURL + "/collections/" + client.collection
	request, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	response, err := client.http.Do(request)
	if err != nil {
		return fmt.Errorf("qdrant unavailable: %w", ErrDependencyUnavailable)
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotFound {
		return client.createCollection(ctx)
	}
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("qdrant collection check HTTP %d", response.StatusCode)
	}
	var body struct {
		Result struct {
			Config struct {
				Params struct {
					Vectors struct {
						Size     int    `json:"size"`
						Distance string `json:"distance"`
					} `json:"vectors"`
				} `json:"params"`
			} `json:"config"`
		} `json:"result"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&body); err != nil {
		return err
	}
	if body.Result.Config.Params.Vectors.Size != client.dimension || !strings.EqualFold(body.Result.Config.Params.Vectors.Distance, "Cosine") {
		return fmt.Errorf("qdrant collection mismatch: expected size=%d distance=Cosine, got size=%d distance=%s", client.dimension, body.Result.Config.Params.Vectors.Size, body.Result.Config.Params.Vectors.Distance)
	}
	return nil
}

func (client *QdrantClient) createCollection(ctx context.Context) error {
	body := map[string]any{"vectors": map[string]any{"size": client.dimension, "distance": "Cosine"}}
	return client.requestJSON(ctx, http.MethodPut, "/collections/"+client.collection, body, nil)
}

func (client *QdrantClient) Upsert(ctx context.Context, points []VectorPoint) error {
	items := make([]map[string]any, 0, len(points))
	for _, point := range points {
		items = append(items, map[string]any{"id": point.ID, "vector": point.Vector, "payload": map[string]any{"class_id": point.ClassID, "material_id": point.MaterialID, "knowledge_entry_id": point.KnowledgeEntryID, "chunk_id": point.ID, "chunk_index": point.ChunkIndex}})
	}
	return client.requestJSON(ctx, http.MethodPut, "/collections/"+client.collection+"/points?wait=true", map[string]any{"points": items}, nil)
}

func (client *QdrantClient) Delete(ctx context.Context, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	return client.requestJSON(ctx, http.MethodPost, "/collections/"+client.collection+"/points/delete?wait=true", map[string]any{"points": ids}, nil)
}

func (client *QdrantClient) Search(ctx context.Context, vector []float32, classID int64, limit int, threshold float64) ([]VectorMatch, error) {
	request := map[string]any{"vector": vector, "limit": limit, "score_threshold": threshold, "with_payload": false, "filter": map[string]any{"must": []map[string]any{{"key": "class_id", "match": map[string]any{"value": classID}}}}}
	var response struct {
		Result []struct {
			ID    int64   `json:"id"`
			Score float64 `json:"score"`
		} `json:"result"`
	}
	if err := client.requestJSON(ctx, http.MethodPost, "/collections/"+client.collection+"/points/search", request, &response); err != nil {
		return nil, err
	}
	result := make([]VectorMatch, len(response.Result))
	for index, item := range response.Result {
		result[index] = VectorMatch{ID: item.ID, Score: item.Score}
	}
	return result, nil
}

func (client *QdrantClient) requestJSON(ctx context.Context, method, path string, requestBody, responseBody any) error {
	var reader io.Reader
	if requestBody != nil {
		body, err := json.Marshal(requestBody)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(body)
	}
	request, err := http.NewRequestWithContext(ctx, method, client.baseURL+path, reader)
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := client.http.Do(request)
	if err != nil {
		return fmt.Errorf("qdrant unavailable: %w", ErrDependencyUnavailable)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
		return fmt.Errorf("qdrant HTTP %d: %w", response.StatusCode, ErrDependencyUnavailable)
	}
	if responseBody != nil {
		return json.NewDecoder(io.LimitReader(response.Body, 4<<20)).Decode(responseBody)
	}
	return nil
}
