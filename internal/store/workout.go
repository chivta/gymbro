package store

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"

	"gymbro/internal/apiclient"
	"gymbro/internal/workout"
)

var selectAliasTarget = `SELECT exercise_id FROM exercise_aliases WHERE user_id = $1 AND alias_key = ` + norm("$2::text")

const (
	// upsertWorkout keeps the workout id on conflict and bumps updated_at.
	// xmax = 0 is true only for a freshly inserted row.
	upsertWorkout = `INSERT INTO workouts (user_id, performed_on, workout_type, kcal, protein_g, note, raw_text, source, source_ref)
		VALUES ($1, $2::text::date, NULLIF($3::text, ''), $4, $5, NULLIF($6::text, ''), $7, $8, $9)
		ON CONFLICT (user_id, source, source_ref) DO UPDATE SET
			performed_on = EXCLUDED.performed_on,
			workout_type = EXCLUDED.workout_type,
			kcal         = EXCLUDED.kcal,
			protein_g    = EXCLUDED.protein_g,
			note         = EXCLUDED.note,
			raw_text     = EXCLUDED.raw_text,
			updated_at   = now()
		RETURNING id, (xmax = 0)`

	deleteEntries = `DELETE FROM workout_exercises WHERE workout_id = $1`
	insertEntry   = `INSERT INTO workout_exercises (workout_id, exercise_id, position, name_as_written)
		VALUES ($1, $2, $3, $4) RETURNING id`
	// Weight travels as text and is cast in SQL, never through float64.
	insertSet = `INSERT INTO exercise_sets (workout_exercise_id, position, weight_kg, reps)
		VALUES ($1, $2, $3::text::numeric, $4)`
)

// SaveWorkout upserts a workout on (user, source, source_ref) in one transaction.
// An existing workout keeps its id and has all entries and sets replaced. Entry
// names resolve alias, then exercise, else a new exercise is created; the names
// created by this call are returned.
func (s *Store) SaveWorkout(ctx context.Context, userID int64, req apiclient.SaveWorkoutRequest) (apiclient.SaveWorkoutResponse, error) {
	resp := apiclient.SaveWorkoutResponse{NewExercises: []string{}}

	err := s.inTx(ctx, func(tx pgx.Tx) error {
		err := tx.QueryRow(ctx, upsertWorkout,
			userID, req.PerformedOn, canonicalType(req.Type), req.Kcal, req.ProteinG,
			req.Note, req.RawText, req.Source, req.SourceRef,
		).Scan(&resp.WorkoutID, &resp.Created)
		if err != nil {
			return fmt.Errorf("upsert workout: %w", err)
		}

		_, err = tx.Exec(ctx, deleteEntries, resp.WorkoutID)
		if err != nil {
			return fmt.Errorf("delete entries: %w", err)
		}

		seen := map[int64]bool{}
		for i, entry := range req.Entries {
			exerciseID, created, err := resolveExercise(ctx, tx, userID, entry.Name)
			if err != nil {
				return err
			}
			// Dedupe by id: a name repeated in one workout is created once.
			if created && !seen[exerciseID] {
				seen[exerciseID] = true
				resp.NewExercises = append(resp.NewExercises, entry.Name)
			}

			var entryID int64
			err = tx.QueryRow(ctx, insertEntry, resp.WorkoutID, exerciseID, i+1, entry.Name).Scan(&entryID)
			if err != nil {
				return fmt.Errorf("insert entry: %w", err)
			}
			for j, set := range entry.Sets {
				_, err = tx.Exec(ctx, insertSet, entryID, j+1, set.Weight, set.Reps)
				if err != nil {
					return fmt.Errorf("insert set: %w", err)
				}
			}
		}
		return nil
	})
	if err != nil {
		return apiclient.SaveWorkoutResponse{}, err
	}
	return resp, nil
}

// resolveExercise applies the resolution order: alias target, exercise by key, new exercise.
func resolveExercise(ctx context.Context, tx pgx.Tx, userID int64, name string) (id int64, created bool, err error) {
	err = tx.QueryRow(ctx, selectAliasTarget, userID, name).Scan(&id)
	if err == nil {
		return id, false, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return 0, false, fmt.Errorf("resolve alias: %w", err)
	}
	err = tx.QueryRow(ctx, selectExerciseByName, userID, name).Scan(&id)
	if err == nil {
		return id, false, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return 0, false, fmt.Errorf("resolve exercise: %w", err)
	}
	return ensureExercise(ctx, tx, userID, name)
}

// canonicalType returns the spelling from workout.Types matching t case-insensitively,
// or "" when t is empty (stored as NULL). Requests are validated beforehand.
func canonicalType(t string) string {
	for _, known := range workout.Types {
		if strings.EqualFold(t, known) {
			return known
		}
	}
	return ""
}
