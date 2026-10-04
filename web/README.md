# gymbro web

Chat-style workout feed. Talks to the Go API over REST only, via relative `/api/v1/...`.
Dev: the Vite server proxies `/api/*` to `API_PROXY_TARGET` and injects `Authorization: Bearer $API_SECRET` (server-side env, never in the bundle).
Production: the image only serves static files (SPA fallback). It needs a reverse proxy in front that routes `/api` and adds the auth header. That proxy does not exist yet.
