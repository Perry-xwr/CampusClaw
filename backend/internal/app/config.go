package app

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	Addr           string
	DatabaseDSN    string
	UploadDir      string
	SessionTTL     time.Duration
	CookieSecure   bool
	MaxUploadBytes int64
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
	cookieSecure, err := strconv.ParseBool(env("COOKIE_SECURE", "false"))
	if err != nil {
		return Config{}, fmt.Errorf("COOKIE_SECURE must be true or false")
	}
	return Config{
		Addr:           env("API_ADDR", ":8080"),
		DatabaseDSN:    env("DATABASE_DSN", "campusclaw:campusclaw@tcp(db:3306)/campusclaw?parseTime=true&charset=utf8mb4&collation=utf8mb4_unicode_ci"),
		UploadDir:      env("UPLOAD_DIR", "/data/uploads"),
		SessionTTL:     time.Duration(ttlHours) * time.Hour,
		CookieSecure:   cookieSecure,
		MaxUploadBytes: maxBytes,
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
