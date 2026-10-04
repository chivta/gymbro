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
	// APISecret is the static bearer token clients must present on /v1.
	APISecret string `env:"API_SECRET" validate:"required"`
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
