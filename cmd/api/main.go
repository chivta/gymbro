package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"gymbro/internal/config"
	"gymbro/internal/server"
)

func main() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = zerolog.New(os.Stderr).With().Timestamp().Caller().Logger()
	zerolog.SetGlobalLevel(zerolog.InfoLevel)

	cfg, err := config.Load()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to load config")
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	err = server.Run(ctx, cfg)
	if err != nil {
		log.Fatal().Err(err).Msg("server exited with error")
	}
}
