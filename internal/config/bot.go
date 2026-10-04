package config

import (
	"fmt"

	"github.com/caarlos0/env/v11"
	"github.com/go-playground/validator/v10"
	"github.com/joho/godotenv"
)

// BotConfig is the Telegram bot's config. Token and APISecret are secrets:
// never log the struct.
type BotConfig struct {
	Token      string `env:"BOT_TOKEN"    validate:"required"`
	APIBaseURL string `env:"API_BASE_URL" validate:"required,url"`
	APISecret  string `env:"API_SECRET"   validate:"required"`
	// DBPath is the bot's SQLite file (first-save times of workouts).
	DBPath string `env:"BOT_DB_PATH" validate:"required"`
	// AllowedTelegramUserID is the only Telegram user the bot talks to.
	AllowedTelegramUserID int64 `env:"ALLOWED_TELEGRAM_USER_ID" validate:"required,gt=0"`
}

// LoadBot reads .env if present, falls back to OS env, and validates the result.
// Callers exit on error: the process must not start with a bad config.
func LoadBot() (BotConfig, error) {
	// A missing .env is fine, the variables then come from the OS environment.
	_ = godotenv.Load()

	var cfg BotConfig
	err := env.Parse(&cfg)
	if err != nil {
		return BotConfig{}, fmt.Errorf("parse env: %w", err)
	}

	err = validator.New().Struct(cfg)
	if err != nil {
		return BotConfig{}, fmt.Errorf("validate config: %w", err)
	}

	return cfg, nil
}
