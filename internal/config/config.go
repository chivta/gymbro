package config

import (
	"fmt"

	"github.com/caarlos0/env/v11"
	"github.com/go-playground/validator/v10"
	"github.com/joho/godotenv"
)

type Config struct {
	Port        string `env:"PORT"         validate:"required,numeric"`
	DatabaseURL string `env:"DATABASE_URL" validate:"required,url"`
	// APISecret is the static bearer token the bot presents on /v1.
	APISecret string `env:"API_SECRET" validate:"required"`
	// BotUsername is the Telegram bot's username without @, for sign-in deep links.
	BotUsername string `env:"BOT_USERNAME" validate:"required,excludes=@"`
	// CookieSecure sets Secure on auth cookies. Disable only for local HTTP.
	CookieSecure bool `env:"COOKIE_SECURE" envDefault:"true"`
}

// Load reads .env if present, falls back to OS env, and validates the result.
// Callers exit on error: the process must not start with a bad config.
func Load() (Config, error) {
	// A missing .env is fine, the variables then come from the OS environment.
	_ = godotenv.Load()

	var cfg Config
	err := env.Parse(&cfg)
	if err != nil {
		return Config{}, fmt.Errorf("parse env: %w", err)
	}

	err = validator.New().Struct(cfg)
	if err != nil {
		return Config{}, fmt.Errorf("validate config: %w", err)
	}

	return cfg, nil
}
