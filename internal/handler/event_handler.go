package handler

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/daewon/haru/internal/dto"
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
	var req dto.CreateEventRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	input, validationErrors := h.toServiceInput(req)
	if len(validationErrors) > 0 {
		response.ValidationFailed(c, validationErrors)
		return
	}

	event, err := h.svc.CreateEvent(c.Request.Context(), input)
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	response.JSON(c, http.StatusCreated, event)
}

// GetByID handles GET /api/events/:id.
func (h *EventHandler) GetByID(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid event ID format")
		return
	}

	event, err := h.svc.GetEvent(c.Request.Context(), id)
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	response.JSON(c, http.StatusOK, event)
}

// List handles GET /api/events?start=&end=.
func (h *EventHandler) List(c *gin.Context) {
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

	events, err := h.svc.ListEvents(c.Request.Context(), start, end)
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	response.JSON(c, http.StatusOK, dto.EventListResponse{
		Events: events,
		Count:  len(events),
	})
}

// Update handles PUT /api/events/:id.
func (h *EventHandler) Update(c *gin.Context) {
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

	input, validationErrors := h.toServiceInput(req)
	if len(validationErrors) > 0 {
		response.ValidationFailed(c, validationErrors)
		return
	}

	event, err := h.svc.UpdateEvent(c.Request.Context(), id, input)
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	response.JSON(c, http.StatusOK, event)
}

// Delete handles DELETE /api/events/:id.
func (h *EventHandler) Delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid event ID format")
		return
	}

	if err := h.svc.DeleteEvent(c.Request.Context(), id); err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *EventHandler) toServiceInput(req dto.CreateEventRequest) (service.CreateEventInput, []response.ValidationError) {
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

	tz := req.Timezone
	if tz == "" {
		tz = "UTC"
	}

	offsets := req.ReminderOffsets
	if offsets == nil {
		offsets = []int64{}
	}

	return service.CreateEventInput{
		Title:           req.Title,
		StartAt:         startAt,
		EndAt:           endAt,
		AllDay:          req.AllDay,
		Timezone:        tz,
		LocationName:    req.LocationName,
		LocationAddress: req.LocationAddress,
		LocationLat:     req.LocationLat,
		LocationLng:     req.LocationLng,
		ReminderOffsets: offsets,
		Notes:           req.Notes,
	}, nil
}

func (h *EventHandler) handleServiceError(c *gin.Context, err error) {
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
