package token_test

import (
	"regexp"
	"testing"

	"gymbro/internal/token"
)

// startPayload is what Telegram accepts as a /start deep-link payload.
var startPayload = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

const encodedLen = 43

func TestNew(t *testing.T) {
	seen := map[string]bool{}
	for range 100 {
		tok := token.New()
		if len(tok) != encodedLen {
			t.Fatalf("len(%q) = %d, want %d", tok, len(tok), encodedLen)
		}
		if !startPayload.MatchString(tok) {
			t.Fatalf("%q is not a valid /start payload", tok)
		}
		if seen[tok] {
			t.Fatalf("duplicate token %q", tok)
		}
		seen[tok] = true
	}
}

func TestHash(t *testing.T) {
	a, b := token.Hash("x"), token.Hash("x")
	if len(a) != 32 || string(a) != string(b) {
		t.Fatalf("hash not a stable 32-byte digest: %x %x", a, b)
	}
	if string(token.Hash("y")) == string(a) {
		t.Fatal("different inputs hash equal")
	}
}
