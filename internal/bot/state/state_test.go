package state

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestFirstSave(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "bot.db")
	s, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}

	_, ok, err := s.FirstSave(ctx, 1, 2)
	if err != nil || ok {
		t.Fatalf("missing: ok=%v err=%v", ok, err)
	}

	first := time.Unix(1_700_000_000, 0)
	if err := s.SetFirstSave(ctx, 1, 2, first); err != nil {
		t.Fatal(err)
	}
	// A second set must not overwrite.
	if err := s.SetFirstSave(ctx, 1, 2, first.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}

	// Reopen: the value survives a restart.
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	got, ok, err := s.FirstSave(ctx, 1, 2)
	if err != nil || !ok || !got.Equal(first) {
		t.Fatalf("got=%v ok=%v err=%v", got, ok, err)
	}
	_, ok, _ = s.FirstSave(ctx, 1, 3)
	if ok {
		t.Fatal("other message must be missing")
	}
}
