package server

import (
	"crypto/sha256"
	"crypto/subtle"
	"strings"

	"github.com/gin-gonic/gin"

	"gymbro/internal/apperr"
)

const bearerPrefix = "Bearer "

// sharedSecretAuth admits requests carrying `Authorization: Bearer <secret>`.
// It is the only place that knows about the secret: swapping this middleware is
// all it takes to move to per-user auth.
func sharedSecretAuth(secret string) gin.HandlerFunc {
	// Compare fixed-size digests so the comparison does not leak the secret length.
	want := sha256.Sum256([]byte(secret))

	return func(c *gin.Context) {
		token, ok := strings.CutPrefix(c.GetHeader("Authorization"), bearerPrefix)
		got := sha256.Sum256([]byte(token))
		if !ok || subtle.ConstantTimeCompare(got[:], want[:]) != 1 {
			respondWithError(c, apperr.ErrUnauthorized)
			c.Abort()
			return
		}
		c.Next()
	}
}
