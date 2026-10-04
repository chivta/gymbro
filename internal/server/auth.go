package server

import (
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"gymbro/internal/apperr"
	"gymbro/internal/store"
)

const (
	bearerPrefix = "Bearer "
	principalKey = "principal"
	userIDParam  = "id"
)

// principal is who a request acts as. The zero value is anonymous.
type principal struct {
	// service: the request carries the shared bot secret and may act on any user.
	service bool
	// userID: the request carries a valid session cookie of this user (0 = none).
	userID int64
}

// Authorization is two steps: authenticate puts a principal in the context,
// then a per-group guard (requireService, requireSession, requireUserAccess)
// decides. Handlers never look at auth.
//
// CSRF: session-authenticated routes are GETs plus POST /auth/logout, and the
// session cookie is SameSite=Lax, so cross-site pages cannot send it on a
// state-changing request. Revisit before adding session-authenticated writes.

// authenticate resolves the principal. A present but wrong bearer secret is
// rejected with 401; an unknown or expired session cookie leaves the request
// anonymous so the guard decides.
func authenticate(secret string, sessions *store.Store) gin.HandlerFunc {
	// Compare fixed-size digests so the comparison does not leak the secret length.
	want := sha256.Sum256([]byte(secret))

	return func(c *gin.Context) {
		var p principal
		header := c.GetHeader("Authorization")
		if header != "" {
			tok, ok := strings.CutPrefix(header, bearerPrefix)
			got := sha256.Sum256([]byte(tok))
			if !ok || subtle.ConstantTimeCompare(got[:], want[:]) != 1 {
				abortWithError(c, apperr.ErrUnauthorized)
				return
			}
			p.service = true
		} else {
			cookie, err := c.Cookie(sessionCookie)
			if err == nil && cookie != "" {
				userID, err := sessions.SessionUser(c.Request.Context(), cookie)
				if err != nil && !errors.Is(err, apperr.ErrUnauthorized) {
					abortWithError(c, err)
					return
				}
				p.userID = userID
			}
		}
		c.Set(principalKey, p)
		c.Next()
	}
}

func currentPrincipal(c *gin.Context) principal {
	p, _ := c.Get(principalKey)
	pr, _ := p.(principal)
	return pr
}

// guard turns an authorization decision into middleware.
func guard(decide func(c *gin.Context) error) gin.HandlerFunc {
	return func(c *gin.Context) {
		err := decide(c)
		if err != nil {
			abortWithError(c, err)
			return
		}
		c.Next()
	}
}

// requireService admits only the bot.
func requireService() gin.HandlerFunc {
	return guard(func(c *gin.Context) error {
		p := currentPrincipal(c)
		switch {
		case p.service:
			return nil
		case p.userID != 0:
			return apperr.ErrForbidden
		default:
			return apperr.ErrUnauthorized
		}
	})
}

// requireSession admits only requests with a valid session cookie.
func requireSession() gin.HandlerFunc {
	return guard(func(c *gin.Context) error {
		if currentPrincipal(c).userID == 0 {
			return apperr.ErrUnauthorized
		}
		return nil
	})
}

// requireUserAccess admits the service for any :id and a session only for its own :id.
func requireUserAccess() gin.HandlerFunc {
	return guard(func(c *gin.Context) error {
		return authorizeUser(currentPrincipal(c), c.Param(userIDParam))
	})
}

// authorizeUser is the decision of requireUserAccess. A malformed id from the
// service passes through so the handler answers 400.
func authorizeUser(p principal, pathID string) error {
	switch {
	case p.service:
		return nil
	case p.userID == 0:
		return apperr.ErrUnauthorized
	case pathID != strconv.FormatInt(p.userID, 10):
		return apperr.ErrForbidden
	default:
		return nil
	}
}

func abortWithError(c *gin.Context, err error) {
	respondWithError(c, err)
	c.Abort()
}
