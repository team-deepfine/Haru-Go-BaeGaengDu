package handler

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/daewon/haru/internal/dto"
	"github.com/daewon/haru/internal/middleware"
	"github.com/daewon/haru/internal/model"
	"github.com/daewon/haru/internal/service"
	"github.com/daewon/haru/pkg/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// EventHandler handles HTTP requests for events.
type EventHandler struct {
	svc service.EventService
}

// NewEventHandler creates a new EventHandler.
func NewEventHandler(svc service.EventService) *EventHandler {
	return &EventHandler{svc: svc}
}

// RegisterRoutes registers event routes on a Gin router group.
func (h *EventHandler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.POST("/events", h.Create)
	rg.GET("/events/:id", h.GetByID)
	rg.GET("/events", h.List)
	rg.PUT("/events/:id", h.Update)
	rg.DELETE("/events/:id", h.Delete)
}

// Create handles POST /api/events.
func (h *EventHandler) Create(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, "authentication required")
		return
	}

	var req dto.CreateEventRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	input, validationErrors := toServiceInput(req)
	if len(validationErrors) > 0 {
		response.ValidationFailed(c, validationErrors)
		return
	}

	event, err := h.svc.CreateEvent(c.Request.Context(), userID, input)
	if err != nil {
		handleServiceError(c, err)
		return
	}

	response.JSON(c, http.StatusCreated, dto.ToEventResponse(event))
}

// GetByID handles GET /api/events/:id.
func (h *EventHandler) GetByID(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, "authentication required")
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid event ID format")
		return
	}

	event, err := h.svc.GetEvent(c.Request.Context(), userID, id)
	if err != nil {
		handleServiceError(c, err)
		return
	}

	response.JSON(c, http.StatusOK, dto.ToEventResponse(event))
}

// List handles GET /api/events?start=&end=.
func (h *EventHandler) List(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, "authentication required")
		return
	}

	startStr := c.Query("start")
	endStr := c.Query("end")

	if startStr == "" || endStr == "" {
		response.Error(c, http.StatusBadRequest, "start and end query parameters are required")
		return
	}

	start, err := time.Parse(time.RFC3339, startStr)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid start time format (use ISO-8601)")
		return
	}
	end, err := time.Parse(time.RFC3339, endStr)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid end time format (use ISO-8601)")
		return
	}

	events, err := h.svc.ListEvents(c.Request.Context(), userID, start, end)
	if err != nil {
		handleServiceError(c, err)
		return
	}

	response.JSON(c, http.StatusOK, dto.ToEventListResponse(events))
}

// Update handles PUT /api/events/:id.
func (h *EventHandler) Update(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, "authentication required")
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid event ID format")
		return
	}

	var req dto.UpdateEventRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	input, validationErrors := toServiceInput(req)
	if len(validationErrors) > 0 {
		response.ValidationFailed(c, validationErrors)
		return
	}

	event, err := h.svc.UpdateEvent(c.Request.Context(), userID, id, input)
	if err != nil {
		handleServiceError(c, err)
		return
	}

	response.JSON(c, http.StatusOK, dto.ToEventResponse(event))
}

// Delete handles DELETE /api/events/:id.
func (h *EventHandler) Delete(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, "authentication required")
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid event ID format")
		return
	}

	if err := h.svc.DeleteEvent(c.Request.Context(), userID, id); err != nil {
		handleServiceError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

func toServiceInput(req dto.CreateEventRequest) (service.CreateEventInput, []response.ValidationError) {
	var errs []response.ValidationError

	startAt, err := time.Parse(time.RFC3339, req.StartAt)
	if err != nil {
		errs = append(errs, response.ValidationError{Field: "startAt", Message: "invalid ISO-8601 format"})
	}
	endAt, err := time.Parse(time.RFC3339, req.EndAt)
	if err != nil {
		errs = append(errs, response.ValidationError{Field: "endAt", Message: "invalid ISO-8601 format"})
	}

	if len(errs) > 0 {
		return service.CreateEventInput{}, errs
	}

	return service.CreateEventInput{
		Title:           req.Title,
		StartAt:         startAt,
		EndAt:           endAt,
		AllDay:          req.AllDay,
		Timezone:        req.Timezone,
		LocationName:    req.LocationName,
		LocationAddress: req.LocationAddress,
		LocationLat:     req.LocationLat,
		LocationLng:     req.LocationLng,
		ReminderOffsets: req.ReminderOffsets,
		Notes:           req.Notes,
	}, nil
}

func handleServiceError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, model.ErrEventNotFound):
		response.Error(c, http.StatusNotFound, err.Error())
	case errors.Is(err, model.ErrTitleRequired),
		errors.Is(err, model.ErrInvalidTimeRange),
		errors.Is(err, model.ErrInvalidTimezone):
		response.Error(c, http.StatusBadRequest, err.Error())
	default:
		slog.Error("internal error", "error", err)
		response.Error(c, http.StatusInternalServerError, "internal server error")
	}
}
