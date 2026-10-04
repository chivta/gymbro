package bot

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/rs/zerolog/log"
)

const (
	// healthAddr serves the liveness/readiness probe; the bot has no other HTTP surface.
	healthAddr            = ":8081"
	healthPath            = "/health"
	healthReadTimeout     = 5 * time.Second
	healthShutdownTimeout = 5 * time.Second
)

// serveHealth answers 200 on healthPath until ctx is cancelled. It runs only
// after the Telegram client was created, so a 200 means the token was accepted.
func serveHealth(ctx context.Context, addr string) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET "+healthPath, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	srv := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: healthReadTimeout}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), healthShutdownTimeout)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	err := srv.ListenAndServe()
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Error().Err(err).Str("addr", addr).Msg("health server failed")
	}
}
