// Package apperr defines the typed errors that cross from the store to the HTTP
// layer. An AppError carries an HTTP status and a machine-readable code, never a
// human message; messages live in the frontend.
package apperr

import (
	"net/http"

	"gymbro/internal/apiclient"
)

type AppError struct {
	Status int
	Code   string
}

func (e *AppError) Error() string { return e.Code }

var (
	ErrInvalidRequest   = &AppError{Status: http.StatusBadRequest, Code: apiclient.CodeInvalidRequest}
	ErrUnauthorized     = &AppError{Status: http.StatusUnauthorized, Code: apiclient.CodeUnauthorized}
	ErrUserNotFound     = &AppError{Status: http.StatusNotFound, Code: apiclient.CodeUserNotFound}
	ErrSameExercise     = &AppError{Status: http.StatusUnprocessableEntity, Code: apiclient.CodeSameExercise}
	ErrExerciseNotFound = &AppError{Status: http.StatusNotFound, Code: apiclient.CodeExerciseNotFound}
	ErrBadNameIsAlias   = &AppError{Status: http.StatusConflict, Code: apiclient.CodeBadNameIsAlias}
	// ErrConflict is an unexpected uniqueness/FK clash, typically a concurrent write; retrying may succeed.
	ErrConflict = &AppError{Status: http.StatusConflict, Code: apiclient.CodeConflict}
	ErrInternal = &AppError{Status: http.StatusInternalServerError, Code: apiclient.CodeInternal}
)
