package server

import (
	"errors"
	"testing"

	"gymbro/internal/apperr"
)

func TestAuthorizeUser(t *testing.T) {
	tests := []struct {
		name   string
		p      principal
		pathID string
		want   error
	}{
		{"service any id", principal{service: true}, "2", nil},
		{"service malformed id", principal{service: true}, "x", nil},
		{"session own id", principal{userID: 1}, "1", nil},
		{"session other id", principal{userID: 1}, "2", apperr.ErrForbidden},
		{"session malformed id", principal{userID: 1}, "01", apperr.ErrForbidden},
		{"anonymous", principal{}, "1", apperr.ErrUnauthorized},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := authorizeUser(tt.p, tt.pathID)
			if !errors.Is(got, tt.want) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
		})
	}
}
