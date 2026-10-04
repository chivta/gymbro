// Package store is the Postgres-backed persistence of users, workouts and
// exercises. Each method is one transaction and returns apperr sentinels for
// expected failures; anything else is an unclassified error for the caller to log.
package store

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"gymbro/internal/apperr"
)

const (
	pgUniqueViolation     = "23505"
	pgForeignKeyViolation = "23503"
	userFKSuffix          = "_user_id_fkey"
)

type Store struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

// norm returns the SQL expression that normalizes a text parameter exactly like the
// generated name_key / alias_key columns, so the database stays authoritative.
func norm(param string) string {
	return `lower(regexp_replace(btrim(` + param + `), '\s+', ' ', 'g'))`
}

// inTx runs fn in a transaction, committing on nil and rolling back otherwise.
// Postgres errors are translated to apperr sentinels.
func (s *Store) inTx(ctx context.Context, fn func(tx pgx.Tx) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	err = fn(tx)
	if err != nil {
		return mapPgError(err)
	}

	err = tx.Commit(ctx)
	if err != nil {
		return mapPgError(fmt.Errorf("commit tx: %w", err))
	}
	return nil
}

// mapPgError turns unhandled unique/foreign-key violations into typed errors.
// Call sites that know more (e.g. alias insert) translate before this runs.
func mapPgError(err error) error {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return err
	}
	switch pgErr.Code {
	case pgUniqueViolation:
		return apperr.ErrConflict
	case pgForeignKeyViolation:
		if strings.HasSuffix(pgErr.ConstraintName, userFKSuffix) {
			return apperr.ErrUserNotFound
		}
		return apperr.ErrConflict
	}
	return err
}

// isUniqueViolation reports whether err is a Postgres unique violation.
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation
}
