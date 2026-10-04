// Package apiclient holds the wire types of the gymbro HTTP API and a thin client
// for it. The server imports the types, so both sides share one definition.
package apiclient

// Error codes returned by the API as {"error": "<code>"}.
const (
	CodeInvalidRequest = "invalid_request"
	CodeUnauthorized   = "unauthorized"
	CodeUserNotFound   = "user_not_found"
	CodeSameExercise   = "same_exercise"
	CodeBadNameIsAlias = "bad_name_is_alias"
	CodeConflict       = "conflict"
	CodeInternal       = "internal"
)

// Outcomes of a replace operation.
const (
	OutcomeRenamed   = "renamed"
	OutcomeMerged    = "merged"
	OutcomeAliasOnly = "alias_only"
)

type ResolveIdentityRequest struct {
	Provider   string `json:"provider"    validate:"required"`
	ExternalID string `json:"external_id" validate:"required"`
}

type ResolveIdentityResponse struct {
	UserID int64 `json:"user_id"`
}

// SaveWorkoutRequest is one parsed workout. Type and Note are empty when absent.
// Type is matched case-insensitively against workout.Types and stored canonically.
// PerformedOn is an ISO date (YYYY-MM-DD).
type SaveWorkoutRequest struct {
	PerformedOn string  `json:"performed_on"       validate:"required,datetime=2006-01-02"`
	Type        string  `json:"type,omitempty"     validate:"omitempty,workout_type"`
	Kcal        *int    `json:"kcal,omitempty"     validate:"omitempty,gte=0"`
	ProteinG    *int    `json:"protein_g,omitempty" validate:"omitempty,gte=0"`
	Note        string  `json:"note,omitempty"`
	RawText     string  `json:"raw_text"           validate:"required"`
	Source      string  `json:"source"             validate:"required"`
	SourceRef   string  `json:"source_ref"         validate:"required"`
	Entries     []Entry `json:"entries"            validate:"dive"`
}

// Entry is one exercise line; its position is its index in the list. Sets may be empty.
type Entry struct {
	Name string `json:"name" validate:"required"`
	Sets []Set  `json:"sets" validate:"dive"`
}

// Set is one set; its position is its index in the list. Weight is a decimal
// string in kg with at most two fractional digits, never a float.
type Set struct {
	Weight string `json:"weight" validate:"required,weight"`
	Reps   int    `json:"reps"   validate:"gte=1"`
}

// SaveWorkoutResponse reports the upsert result. NewExercises lists the exercise
// names created by this save (deduplicated), so typos can be fixed right away.
type SaveWorkoutResponse struct {
	WorkoutID    int64    `json:"workout_id"`
	Created      bool     `json:"created"`
	NewExercises []string `json:"new_exercises"`
}

type Exercise struct {
	ID      int64    `json:"id"`
	Name    string   `json:"name"`
	Aliases []string `json:"aliases"`
}

type ListExercisesResponse struct {
	Exercises []Exercise `json:"exercises"`
}

type ReplaceExerciseRequest struct {
	BadName     string `json:"bad_name"     validate:"required"`
	CorrectName string `json:"correct_name" validate:"required"`
}

// ReplaceExerciseResponse: Outcome is one of the Outcome* constants, Exercise the
// resulting (correct) exercise including the new alias.
type ReplaceExerciseResponse struct {
	Outcome  string   `json:"outcome"`
	Exercise Exercise `json:"exercise"`
}
