package store

import (
	"encoding/base64"
	"errors"
	"testing"

	"gymbro/internal/apperr"
)

func TestCursorRoundTrip(t *testing.T) {
	cursor := encodeCursor("2026-10-03", 333)
	if cursor != "MjAyNi0xMC0wMzozMzM" {
		t.Fatalf("cursor = %q", cursor)
	}
	date, id, err := decodeCursor(cursor)
	if err != nil || date != "2026-10-03" || id != 333 {
		t.Fatalf("got %q %d %v", date, id, err)
	}
}

func TestDecodeCursorRejectsGarbage(t *testing.T) {
	enc := func(s string) string { return base64.RawURLEncoding.EncodeToString([]byte(s)) }
	tests := []struct{ name, cursor string }{
		{"not base64", "!!!"},
		{"padded base64", "MjAyNi0xMC0wMzozMzM="},
		{"no separator", enc("2026-10-03")},
		{"bad date", enc("2026-13-40:5")},
		{"bad id", enc("2026-10-03:abc")},
		{"zero id", enc("2026-10-03:0")},
		{"negative id", enc("2026-10-03:-4")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := decodeCursor(tt.cursor)
			if !errors.Is(err, apperr.ErrInvalidRequest) {
				t.Fatalf("got %v, want ErrInvalidRequest", err)
			}
		})
	}
}
