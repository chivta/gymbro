# gymbro web

Chat-style workout feed. Talks to the Go API over REST only, via relative `/api/v1/...`.
Auth: on load the app calls `/api/auth/me`; signed out, it shows "Sign in with Telegram" (bot deep link plus polling). The API's HttpOnly session cookie authenticates every later request.
Dev: the Vite server proxies `/api/*` to `API_PROXY_TARGET`, stripping the `/api` prefix. Cookies pass through unchanged and bind to the page origin.
Production: nginx serves the SPA (with fallback) and proxies `/api/*` to the API service on the same origin, so the session cookies apply (see `nginx.conf`).
