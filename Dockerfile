# Dev: hot reload with air, source is bind-mounted by docker-compose.
FROM golang:1.27-alpine AS dev
WORKDIR /app
RUN go install github.com/air-verse/air@v1.67.4
COPY go.mod go.sum ./
RUN go mod download
CMD ["air"]

# Builder: static production binary. CMD picks the binary: api (default) or bot.
FROM golang:1.27-alpine AS builder
ARG CMD=api
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /app-bin ./cmd/${CMD}

# Production: binary only, non-root.
FROM scratch AS production
COPY --from=builder /app-bin /app-bin
USER 65534:65534
ENTRYPOINT ["/app-bin"]
