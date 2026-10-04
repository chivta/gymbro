package server

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"

	"gymbro/internal/apiclient"
	"gymbro/internal/apperr"
	"gymbro/internal/store"
)

// handlers holds the HTTP handlers of the /v1 API. They know nothing about auth.
type handlers struct {
	store    *store.Store
	validate *validator.Validate
}

type userURI struct {
	ID int64 `uri:"id" validate:"required,gt=0"`
}

// register mounts the routes on the (already authenticated) /v1 group.
func (h *handlers) register(v1 *gin.RouterGroup) {
	v1.POST("/identities/resolve", h.resolveIdentity)
	v1.POST("/users/:id/workouts", h.saveWorkout)
	v1.GET("/users/:id/exercises", h.listExercises)
	v1.POST("/users/:id/exercises/replace", h.replaceExercise)
}

func (h *handlers) resolveIdentity(c *gin.Context) {
	var req apiclient.ResolveIdentityRequest
	if !h.bindJSON(c, &req) || !h.valid(c, req) {
		return
	}

	userID, err := h.store.ResolveIdentity(c.Request.Context(), req.Provider, req.ExternalID)
	if err != nil {
		respondWithError(c, err)
		return
	}
	c.JSON(http.StatusOK, apiclient.ResolveIdentityResponse{UserID: userID})
}

// saveWorkout answers 201 when the workout was created and 200 when an existing one was replaced.
func (h *handlers) saveWorkout(c *gin.Context) {
	userID, ok := h.bindUser(c)
	if !ok {
		return
	}
	var req apiclient.SaveWorkoutRequest
	if !h.bindJSON(c, &req) {
		return
	}
	for i := range req.Entries {
		req.Entries[i].Name = trimName(req.Entries[i].Name)
	}
	if !h.valid(c, req) {
		return
	}

	resp, err := h.store.SaveWorkout(c.Request.Context(), userID, req)
	if err != nil {
		respondWithError(c, err)
		return
	}
	status := http.StatusOK
	if resp.Created {
		status = http.StatusCreated
	}
	c.JSON(status, resp)
}

func (h *handlers) listExercises(c *gin.Context) {
	userID, ok := h.bindUser(c)
	if !ok {
		return
	}

	exercises, err := h.store.ListExercises(c.Request.Context(), userID)
	if err != nil {
		respondWithError(c, err)
		return
	}
	c.JSON(http.StatusOK, apiclient.ListExercisesResponse{Exercises: exercises})
}

func (h *handlers) replaceExercise(c *gin.Context) {
	userID, ok := h.bindUser(c)
	if !ok {
		return
	}
	var req apiclient.ReplaceExerciseRequest
	if !h.bindJSON(c, &req) {
		return
	}
	req.BadName = trimName(req.BadName)
	req.CorrectName = trimName(req.CorrectName)
	if !h.valid(c, req) {
		return
	}

	resp, err := h.store.ReplaceExercise(c.Request.Context(), userID, req.BadName, req.CorrectName)
	if err != nil {
		respondWithError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// bindUser reads and validates the :id path parameter, answering 400 on failure.
func (h *handlers) bindUser(c *gin.Context) (int64, bool) {
	var uri userURI
	err := c.ShouldBindUri(&uri)
	if err != nil {
		respondWithError(c, fmt.Errorf("bind uri: %v: %w", err, apperr.ErrInvalidRequest))
		return 0, false
	}
	if !h.valid(c, uri) {
		return 0, false
	}
	return uri.ID, true
}

// bindJSON decodes the body, answering 400 on malformed JSON. Callers validate
// afterwards with valid, once any normalization is done.
func (h *handlers) bindJSON(c *gin.Context, req any) bool {
	err := c.ShouldBindJSON(req)
	if err != nil {
		respondWithError(c, fmt.Errorf("bind json: %v: %w", err, apperr.ErrInvalidRequest))
		return false
	}
	return true
}

// valid runs struct validation and answers 400 on failure.
func (h *handlers) valid(c *gin.Context, req any) bool {
	err := h.validate.Struct(req)
	if err != nil {
		respondWithError(c, fmt.Errorf("validate: %v: %w", err, apperr.ErrInvalidRequest))
		return false
	}
	return true
}
