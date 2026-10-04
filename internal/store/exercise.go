package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"gymbro/internal/apiclient"
	"gymbro/internal/apperr"
)

// Queries comparing names go through norm so the database normalizes them.
var (
	sameKey     = `SELECT ` + norm("$1::text") + ` = ` + norm("$2::text")
	aliasExists = `SELECT 1 FROM exercise_aliases WHERE user_id = $1 AND alias_key = ` + norm("$2::text") + ` FOR UPDATE`

	// lockExerciseByAlias / lockExerciseByName return the exercise a name resolves to.
	lockExerciseByAlias = `SELECT e.id FROM exercise_aliases a JOIN exercises e ON e.id = a.exercise_id
		WHERE a.user_id = $1 AND a.alias_key = ` + norm("$2::text") + ` FOR UPDATE OF a, e`
	selectExerciseByName = `SELECT id FROM exercises WHERE user_id = $1 AND name_key = ` + norm("$2::text")
	lockExerciseByName   = `SELECT id FROM exercises WHERE user_id = $1 AND name_key = ` + norm("$2::text") + ` FOR UPDATE`
)

const (
	listExercises = `SELECT e.id, e.name,
			ARRAY(SELECT a.alias FROM exercise_aliases a WHERE a.exercise_id = e.id ORDER BY a.alias_key)
		FROM exercises e WHERE e.user_id = $1 ORDER BY e.name_key`

	getExercise = `SELECT e.id, e.name,
			ARRAY(SELECT a.alias FROM exercise_aliases a WHERE a.exercise_id = e.id ORDER BY a.alias_key)
		FROM exercises e WHERE e.id = $1`

	// lockUser serializes replace operations of one user. NO KEY UPDATE does not
	// block inserts that only reference the user, so workout saves are unaffected.
	lockUser = `SELECT id FROM users WHERE id = $1 FOR NO KEY UPDATE`

	// insertExercise is conflict-safe: no row is returned when the key already exists.
	insertExercise = `INSERT INTO exercises (user_id, name) VALUES ($1, $2)
		ON CONFLICT (user_id, name_key) DO NOTHING RETURNING id`

	renameExercise = `UPDATE exercises SET name = $2 WHERE id = $1`
	repointEntries = `UPDATE workout_exercises SET exercise_id = $2 WHERE exercise_id = $1`
	repointAliases = `UPDATE exercise_aliases SET exercise_id = $2 WHERE exercise_id = $1`
	deleteExercise = `DELETE FROM exercises WHERE id = $1`
	insertAlias    = `INSERT INTO exercise_aliases (user_id, exercise_id, alias) VALUES ($1, $2, $3)`
)

// ListExercises returns all exercises of a user ordered by name, each with its aliases.
func (s *Store) ListExercises(ctx context.Context, userID int64) ([]apiclient.Exercise, error) {
	rows, err := s.pool.Query(ctx, listExercises, userID)
	if err != nil {
		return nil, fmt.Errorf("list exercises: %w", err)
	}
	defer rows.Close()

	exercises := []apiclient.Exercise{}
	for rows.Next() {
		var ex apiclient.Exercise
		err = rows.Scan(&ex.ID, &ex.Name, &ex.Aliases)
		if err != nil {
			return nil, fmt.Errorf("scan exercise: %w", err)
		}
		exercises = append(exercises, ex)
	}
	err = rows.Err()
	if err != nil {
		return nil, fmt.Errorf("list exercises: %w", err)
	}
	return exercises, nil
}

// ReplaceExercise fixes a typo: it makes badName an alias of correctName and moves
// everything that used the bad exercise over to the correct one (rename when the
// correct exercise does not exist yet, merge when both do, alias only when the bad
// name is not an exercise). Returns ErrSameExercise when both names normalize to
// one key, ErrBadNameIsAlias when badName is already an alias.
func (s *Store) ReplaceExercise(ctx context.Context, userID int64, badName, correctName string) (apiclient.ReplaceExerciseResponse, error) {
	var resp apiclient.ReplaceExerciseResponse

	err := s.inTx(ctx, func(tx pgx.Tx) error {
		var lockedID int64
		err := tx.QueryRow(ctx, lockUser, userID).Scan(&lockedID)
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.ErrUserNotFound
		}
		if err != nil {
			return fmt.Errorf("lock user: %w", err)
		}

		var same bool
		err = tx.QueryRow(ctx, sameKey, badName, correctName).Scan(&same)
		if err != nil {
			return fmt.Errorf("compare keys: %w", err)
		}
		if same {
			return apperr.ErrSameExercise
		}

		var one int
		err = tx.QueryRow(ctx, aliasExists, userID, badName).Scan(&one)
		if err == nil {
			return apperr.ErrBadNameIsAlias
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("check bad alias: %w", err)
		}

		badID, badFound, err := lockOne(ctx, tx, lockExerciseByName, userID, badName)
		if err != nil {
			return fmt.Errorf("find bad exercise: %w", err)
		}

		// The correct name may itself be an alias; then it means the alias target.
		correctID, correctFound, err := lockOne(ctx, tx, lockExerciseByAlias, userID, correctName)
		if err != nil {
			return fmt.Errorf("find correct exercise by alias: %w", err)
		}
		if !correctFound {
			correctID, correctFound, err = lockOne(ctx, tx, lockExerciseByName, userID, correctName)
			if err != nil {
				return fmt.Errorf("find correct exercise: %w", err)
			}
		}
		if badFound && correctFound && badID == correctID {
			// correctName is an alias of the bad exercise: same exercise.
			return apperr.ErrSameExercise
		}

		switch {
		case badFound && correctFound:
			resp.Outcome = apiclient.OutcomeMerged
			_, err = tx.Exec(ctx, repointEntries, badID, correctID)
			if err != nil {
				return fmt.Errorf("repoint entries: %w", err)
			}
			_, err = tx.Exec(ctx, repointAliases, badID, correctID)
			if err != nil {
				return fmt.Errorf("repoint aliases: %w", err)
			}
			_, err = tx.Exec(ctx, deleteExercise, badID)
			if err != nil {
				return fmt.Errorf("delete bad exercise: %w", err)
			}
		case badFound:
			resp.Outcome = apiclient.OutcomeRenamed
			correctID = badID
			_, err = tx.Exec(ctx, renameExercise, badID, correctName)
			if err != nil {
				return fmt.Errorf("rename exercise: %w", err)
			}
		case correctFound:
			resp.Outcome = apiclient.OutcomeAliasOnly
		default:
			// An alias must point at an exercise, so create the correct one.
			resp.Outcome = apiclient.OutcomeAliasOnly
			correctID, _, err = ensureExercise(ctx, tx, userID, correctName)
			if err != nil {
				return fmt.Errorf("create correct exercise: %w", err)
			}
		}

		_, err = tx.Exec(ctx, insertAlias, userID, correctID, badName)
		if isUniqueViolation(err) {
			return apperr.ErrBadNameIsAlias
		}
		if err != nil {
			return fmt.Errorf("insert alias: %w", err)
		}

		err = tx.QueryRow(ctx, getExercise, correctID).Scan(&resp.Exercise.ID, &resp.Exercise.Name, &resp.Exercise.Aliases)
		if err != nil {
			return fmt.Errorf("read result: %w", err)
		}
		return nil
	})
	if err != nil {
		return apiclient.ReplaceExerciseResponse{}, err
	}
	return resp, nil
}

// lockOne runs a locking single-id query; found is false when no row matches.
func lockOne(ctx context.Context, tx pgx.Tx, query string, userID int64, name string) (id int64, found bool, err error) {
	err = tx.QueryRow(ctx, query, userID, name).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	return id, true, nil
}

// ensureExercise returns the id of the user's exercise with this name, creating it
// (display name as given) if missing. created is true only for the caller that
// inserted it; a concurrent creator makes the insert a no-op and the row is read back.
func ensureExercise(ctx context.Context, tx pgx.Tx, userID int64, name string) (id int64, created bool, err error) {
	err = tx.QueryRow(ctx, insertExercise, userID, name).Scan(&id)
	if err == nil {
		return id, true, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return 0, false, fmt.Errorf("insert exercise: %w", err)
	}
	err = tx.QueryRow(ctx, selectExerciseByName, userID, name).Scan(&id)
	if err != nil {
		return 0, false, fmt.Errorf("select exercise: %w", err)
	}
	return id, false, nil
}
