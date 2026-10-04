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

// handlers holds the HTTP handlers. They know nothing about authorization: the
// route groups they are mounted on carry the guards (see auth.go).
type handlers struct {
	store    *store.Store
	validate *validator.Validate
	// botUsername builds the sign-in deep link, without the @.
	botUsername string
	// cookieSecure sets the Secure attribute on auth cookies (false only for local HTTP).
	cookieSecure bool
}

type userURI struct {
	ID int64 `uri:"id" validate:"required,gt=0"`
}

// listWorkoutsQuery: limit defaults to 20 and is bounded to 1..100 (struct tags
// cannot reference constants); before is an opaque cursor checked by the store.
type listWorkoutsQuery struct {
	Limit  int    `form:"limit,default=20" validate:"gte=1,lte=100"`
	Before string `form:"before"`
}

// register mounts the /v1 routes. service admits only the bot; userAccess
// admits the bot or a session of the :id user.
func (h *handlers) register(service, userAccess *gin.RouterGroup) {
	service.POST("/identities/resolve", h.resolveIdentity)
	service.POST("/users/:id/workouts", h.saveWorkout)
	service.GET("/users/:id/exercises", h.listExercises)
	service.POST("/users/:id/exercises/replace", h.replaceExercise)
	service.POST("/auth/telegram/confirm", h.confirmTelegram)
	userAccess.GET("/users/:id/workouts", h.listWorkouts)
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

// listWorkouts serves one keyset page of workouts, newest first.
func (h *handlers) listWorkouts(c *gin.Context) {
	userID, ok := h.bindUser(c)
	if !ok {
		return
	}
	var q listWorkoutsQuery
	err := c.ShouldBindQuery(&q)
	if err != nil {
		respondWithError(c, fmt.Errorf("bind query: %v: %w", err, apperr.ErrInvalidRequest))
		return
	}
	if !h.valid(c, q) {
		return
	}

	resp, err := h.store.ListWorkouts(c.Request.Context(), userID, q.Limit, q.Before)
	if err != nil {
		respondWithError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
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
