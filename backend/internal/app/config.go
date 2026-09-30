package app

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	Addr               string
	DatabaseDSN        string
	UploadDir          string
	SessionTTL         time.Duration
	MaxUploadBytes     int64
	QdrantURL          string
	QdrantCollection   string
	EmbeddingBaseURL   string
	EmbeddingAPIKey    string
	EmbeddingModel     string
	EmbeddingDimension int
	ChatBaseURL        string
	ChatAPIKey         string
	ChatModel          string
	ModelTimeout       time.Duration
	VectorMinScore     float64
	KeywordMinScore    float64
}

func ConfigFromEnv() (Config, error) {
	ttlHours, err := intEnv("SESSION_TTL_HOURS", 24)
	if err != nil || ttlHours < 1 {
		return Config{}, fmt.Errorf("SESSION_TTL_HOURS must be a positive integer")
	}
	maxBytes, err := int64Env("MAX_UPLOAD_BYTES", 1<<20)
	if err != nil || maxBytes < 1 {
		return Config{}, fmt.Errorf("MAX_UPLOAD_BYTES must be a positive integer")
	}
	dimension, err := intEnv("EMBEDDING_DIMENSION", 16)
	if err != nil || dimension < 1 {
		return Config{}, fmt.Errorf("EMBEDDING_DIMENSION must be a positive integer")
	}
	timeoutSeconds, err := intEnv("MODEL_TIMEOUT_SECONDS", 10)
	if err != nil || timeoutSeconds < 1 || timeoutSeconds > 120 {
		return Config{}, fmt.Errorf("MODEL_TIMEOUT_SECONDS must be between 1 and 120")
	}
	vectorMin, err := float64Env("VECTOR_MIN_SCORE", 0.35)
	if err != nil || vectorMin < 0 || vectorMin > 1 {
		return Config{}, fmt.Errorf("VECTOR_MIN_SCORE must be between 0 and 1")
	}
	keywordMin, err := float64Env("KEYWORD_MIN_SCORE", 0.01)
	if err != nil || keywordMin < 0 {
		return Config{}, fmt.Errorf("KEYWORD_MIN_SCORE must be non-negative")
	}
	return Config{
		Addr:               env("API_ADDR", ":8080"),
		DatabaseDSN:        env("DATABASE_DSN", "campusclaw:campusclaw@tcp(db:3306)/campusclaw?parseTime=true&charset=utf8mb4&collation=utf8mb4_unicode_ci"),
		UploadDir:          env("UPLOAD_DIR", "/data/uploads"),
		SessionTTL:         time.Duration(ttlHours) * time.Hour,
		MaxUploadBytes:     maxBytes,
		QdrantURL:          env("QDRANT_URL", "http://qdrant:6333"),
		QdrantCollection:   env("QDRANT_COLLECTION", "campusclaw_chunks"),
		EmbeddingBaseURL:   env("EMBEDDING_BASE_URL", "http://model-mock:8081"),
		EmbeddingAPIKey:    os.Getenv("EMBEDDING_API_KEY"),
		EmbeddingModel:     env("EMBEDDING_MODEL", "campusclaw-mock-embedding"),
		EmbeddingDimension: dimension,
		ChatBaseURL:        env("CHAT_BASE_URL", "http://model-mock:8081"),
		ChatAPIKey:         os.Getenv("CHAT_API_KEY"),
		ChatModel:          env("CHAT_MODEL", "campusclaw-mock-chat"),
		ModelTimeout:       time.Duration(timeoutSeconds) * time.Second,
		VectorMinScore:     vectorMin,
		KeywordMinScore:    keywordMin,
	}, nil
}

func env(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func intEnv(name string, fallback int) (int, error) {
	value, err := strconv.Atoi(env(name, strconv.Itoa(fallback)))
	return value, err
}

func int64Env(name string, fallback int64) (int64, error) {
	value, err := strconv.ParseInt(env(name, strconv.FormatInt(fallback, 10)), 10, 64)
	return value, err
}

func float64Env(name string, fallback float64) (float64, error) {
	return strconv.ParseFloat(env(name, strconv.FormatFloat(fallback, 'f', -1, 64)), 64)
}
