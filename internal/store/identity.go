package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

const (
	selectIdentity = `SELECT user_id FROM user_identities WHERE provider = $1 AND external_id = $2`
	insertUser     = `INSERT INTO users DEFAULT VALUES RETURNING id`
	insertIdentity = `INSERT INTO user_identities (user_id, provider, external_id) VALUES ($1, $2, $3)
		ON CONFLICT (provider, external_id) DO NOTHING RETURNING user_id`
)

// ResolveIdentity returns the user bound to (provider, externalID), creating the
// user and identity if missing. Concurrent callers converge on one user: the loser
// of the identity insert rolls back its new user and reads the winner's.
func (s *Store) ResolveIdentity(ctx context.Context, provider, externalID string) (int64, error) {
	userID, err := s.findIdentity(ctx, provider, externalID)
	if err == nil {
		return userID, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return 0, err
	}

	created := false
	err = s.inTx(ctx, func(tx pgx.Tx) error {
		var newID int64
		err := tx.QueryRow(ctx, insertUser).Scan(&newID)
		if err != nil {
			return fmt.Errorf("insert user: %w", err)
		}

		err = tx.QueryRow(ctx, insertIdentity, newID, provider, externalID).Scan(&userID)
		if errors.Is(err, pgx.ErrNoRows) {
			// Lost the race. Returning rolls the new user back.
			return errLostRace
		}
		if err != nil {
			return fmt.Errorf("insert identity: %w", err)
		}
		created = true
		return nil
	})
	if created {
		return userID, nil
	}
	if !errors.Is(err, errLostRace) {
		return 0, err
	}
	return s.findIdentity(ctx, provider, externalID)
}

var errLostRace = errors.New("identity created concurrently")

func (s *Store) findIdentity(ctx context.Context, provider, externalID string) (int64, error) {
	var userID int64
	err := s.pool.QueryRow(ctx, selectIdentity, provider, externalID).Scan(&userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, err
		}
		return 0, fmt.Errorf("select identity: %w", err)
	}
	return userID, nil
}
