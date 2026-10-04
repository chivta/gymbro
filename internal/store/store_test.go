package store

import (
	"errors"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"

	"gymbro/internal/apperr"
)

func TestMapPgError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want error
	}{
		{"unique", &pgconn.PgError{Code: pgUniqueViolation}, apperr.ErrConflict},
		{"wrapped unique", fmt.Errorf("x: %w", &pgconn.PgError{Code: pgUniqueViolation}), apperr.ErrConflict},
		{"missing user", &pgconn.PgError{Code: pgForeignKeyViolation, ConstraintName: "workouts_user_id_fkey"}, apperr.ErrUserNotFound},
		{"other fk", &pgconn.PgError{Code: pgForeignKeyViolation, ConstraintName: "workout_exercises_exercise_id_fkey"}, apperr.ErrConflict},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mapPgError(tt.err)
			if !errors.Is(got, tt.want) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
		})
	}

	plain := errors.New("boom")
	if got := mapPgError(plain); got != plain {
		t.Fatalf("unrelated error must pass through, got %v", got)
	}
}
