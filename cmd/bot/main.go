package main

import (
	// Embedded zone database: the container image has none.
	_ "time/tzdata"

	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"gymbro/internal/bot"
	"gymbro/internal/config"
)

func main() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = zerolog.New(os.Stderr).With().Timestamp().Caller().Logger()
	zerolog.SetGlobalLevel(zerolog.InfoLevel)

	cfg, err := config.LoadBot()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to load config")
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	err = bot.Run(ctx, cfg)
	if err != nil {
		log.Fatal().Err(err).Msg("bot exited with error")
	}
}
