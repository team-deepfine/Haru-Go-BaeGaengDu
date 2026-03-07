package config

import (
	"os"

	"github.com/joho/godotenv"
)

// Config holds all application configuration loaded from environment variables.
type Config struct {
	Port     string
	DB       DBConfig
	LogLevel string
	Gemini   GeminiConfig
}

// DBConfig holds database connection settings.
type DBConfig struct {
	URL    string
	Driver string // "postgres" or "sqlite"
}

// GeminiConfig holds Gemini API settings.
type GeminiConfig struct {
	APIKey     string
	Model      string
	Timezone   string
	PromptPath string
}

// Load reads configuration from environment variables (with .env fallback).
func Load() *Config {
	_ = godotenv.Load()

	return &Config{
		Port: getEnv("PORT", "8080"),
		DB: DBConfig{
			URL:    getEnv("DATABASE_URL", ""),
			Driver: getEnv("DB_DRIVER", "sqlite"),
		},
		LogLevel: getEnv("LOG_LEVEL", "info"),
		Gemini: GeminiConfig{
			APIKey:     getEnv("GEMINI_API_KEY", ""),
			Model:      getEnv("GEMINI_MODEL", "gemini-2.5-flash"),
			Timezone:   getEnv("DEFAULT_TIMEZONE", "Asia/Seoul"),
			PromptPath: getEnv("VOICE_PARSE_PROMPT_PATH", ""),
		},
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
