package server

import (
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"gymbro/internal/apiclient"
	"gymbro/internal/apperr"
)

const (
	loginTTL   = 10 * time.Minute
	sessionTTL = 30 * 24 * time.Hour

	// loginCookie binds a login request to the browser that started it, so
	// seeing the deep link alone is not enough to claim the session.
	loginCookie   = "gymbro_login"
	sessionCookie = "gymbro_session"
	cookiePath    = "/"

	telegramProvider = "telegram"
	telegramLinkBase = "https://t.me/"
	startParam       = "start"
)

// Web sign-in. The API knows users and sessions; Telegram only appears as the
// identity provider that confirms a login request (confirmTelegram, mounted on
// the service-only /v1 group).

// registerAuth mounts the /auth routes: public needs no principal, session
// needs a session cookie.
func (h *handlers) registerAuth(public, session *gin.RouterGroup) {
	public.POST("/telegram/login", h.startLogin)
	public.POST("/telegram/poll", h.pollLogin)
	session.GET("/me", h.me)
	session.POST("/logout", h.logout)
}

// startLogin creates a login request: the token goes into the bot deep link,
// the nonce into the login cookie.
func (h *handlers) startLogin(c *gin.Context) {
	req, err := h.store.CreateLoginRequest(c.Request.Context(), loginTTL)
	if err != nil {
		respondWithError(c, err)
		return
	}
	h.setCookie(c, loginCookie, req.Nonce, loginTTL)
	link := telegramLinkBase + h.botUsername + "?" + url.Values{startParam: {req.Token}}.Encode()
	c.JSON(http.StatusOK, apiclient.StartLoginResponse{BotURL: link, ExpiresAt: req.ExpiresAt})
}

// confirmTelegram is called by the bot when a Telegram user opens the deep link.
func (h *handlers) confirmTelegram(c *gin.Context) {
	var req apiclient.ConfirmLoginRequest
	if !h.bindJSON(c, &req) || !h.valid(c, req) {
		return
	}

	ctx := c.Request.Context()
	userID, err := h.store.ResolveIdentity(ctx, telegramProvider, strconv.FormatInt(req.TelegramUserID, 10))
	if err != nil {
		respondWithError(c, err)
		return
	}
	err = h.store.ConfirmLogin(ctx, req.LoginToken, userID)
	if err != nil {
		respondWithError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// pollLogin reports whether the browser's login request was confirmed and, the
// first time it is, swaps the login cookie for a session cookie.
func (h *handlers) pollLogin(c *gin.Context) {
	nonce, err := c.Cookie(loginCookie)
	if err != nil || nonce == "" {
		respondWithError(c, apperr.ErrLoginExpired)
		return
	}

	session, err := h.store.ClaimLogin(c.Request.Context(), nonce, sessionTTL)
	if err != nil {
		respondWithError(c, err)
		return
	}
	if session == nil {
		c.JSON(http.StatusOK, apiclient.PollLoginResponse{Status: apiclient.LoginPending})
		return
	}
	h.setCookie(c, sessionCookie, session.Token, sessionTTL)
	h.clearCookie(c, loginCookie)
	c.JSON(http.StatusOK, apiclient.PollLoginResponse{Status: apiclient.LoginConfirmed, UserID: session.UserID})
}

func (h *handlers) me(c *gin.Context) {
	c.JSON(http.StatusOK, apiclient.MeResponse{UserID: currentPrincipal(c).userID})
}

func (h *handlers) logout(c *gin.Context) {
	// requireSession guarantees the cookie is present.
	tok, _ := c.Cookie(sessionCookie)
	err := h.store.DeleteSession(c.Request.Context(), tok)
	if err != nil {
		respondWithError(c, err)
		return
	}
	h.clearCookie(c, sessionCookie)
	c.Status(http.StatusNoContent)
}

// setCookie sets an HttpOnly, SameSite=Lax cookie on Path / that lives for ttl.
func (h *handlers) setCookie(c *gin.Context, name, value string, ttl time.Duration) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     cookiePath,
		MaxAge:   int(ttl.Seconds()),
		HttpOnly: true,
		Secure:   h.cookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
}

// clearCookie expires a cookie set by setCookie.
func (h *handlers) clearCookie(c *gin.Context, name string) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     name,
		Path:     cookiePath,
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   h.cookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
}
