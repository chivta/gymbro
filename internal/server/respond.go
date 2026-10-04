package server

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"

	"gymbro/internal/apperr"
)

// respondWithError writes {"error": "<code>"} for any error and is the single
// place errors are logged. Unknown errors become 500 internal.
func respondWithError(c *gin.Context, err error) {
	var appErr *apperr.AppError
	if !errors.As(err, &appErr) {
		appErr = apperr.ErrInternal
	}

	event := log.Info()
	if appErr.Status >= http.StatusInternalServerError {
		event = log.Error()
	}
	event.Err(err).Str("method", c.Request.Method).Str("path", c.FullPath()).
		Int("status", appErr.Status).Str("code", appErr.Code).Msg("request failed")

	c.JSON(appErr.Status, gin.H{"error": appErr.Code})
}
