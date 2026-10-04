# gymbro web

Chat-style workout feed. Talks to the Go API over REST only, via relative `/api/v1/...`.
Auth: on load the app calls `/api/auth/me`; signed out, it shows "Sign in with Telegram" (bot deep link plus polling). The API's HttpOnly session cookie authenticates every later request.
Dev: the Vite server proxies `/api/*` to `API_PROXY_TARGET`, stripping the `/api` prefix. Cookies pass through unchanged and bind to the page origin.
Production: the image only serves static files (SPA fallback). It needs a reverse proxy in front that routes `/api/*` to the API on the same origin, so the cookies apply. That proxy does not exist yet.
