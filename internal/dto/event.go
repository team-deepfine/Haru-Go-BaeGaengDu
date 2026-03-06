package dto

import (
	"time"

	"github.com/daewon/haru/internal/model"
)

// CreateEventRequest is the request body for creating an event.
type CreateEventRequest struct {
	Title           string   `json:"title" binding:"required"`
	StartAt         string   `json:"startAt" binding:"required"`
	EndAt           string   `json:"endAt" binding:"required"`
	AllDay          bool     `json:"allDay"`
	Timezone        string   `json:"timezone"`
	LocationName    *string  `json:"locationName"`
	LocationAddress *string  `json:"locationAddress"`
	LocationLat     *float64 `json:"locationLat"`
	LocationLng     *float64 `json:"locationLng"`
	ReminderOffsets []int64  `json:"reminderOffsets"`
	Notes           *string  `json:"notes"`
}

// UpdateEventRequest is the same structure as CreateEventRequest.
type UpdateEventRequest = CreateEventRequest

// EventResponse is the API response representation of an event.
type EventResponse struct {
	ID              string   `json:"id"`
	Title           string   `json:"title"`
	StartAt         string   `json:"startAt"`
	EndAt           string   `json:"endAt"`
	AllDay          bool     `json:"allDay"`
	Timezone        string   `json:"timezone"`
	LocationName    *string  `json:"locationName,omitempty"`
	LocationAddress *string  `json:"locationAddress,omitempty"`
	LocationLat     *float64 `json:"locationLat,omitempty"`
	LocationLng     *float64 `json:"locationLng,omitempty"`
	ReminderOffsets []int64  `json:"reminderOffsets"`
	Notes           *string  `json:"notes,omitempty"`
	CreatedAt       string   `json:"createdAt"`
	UpdatedAt       string   `json:"updatedAt"`
}

// EventListResponse is the response body for listing events.
type EventListResponse struct {
	Events []EventResponse `json:"events"`
	Count  int             `json:"count"`
}

// ToEventResponse converts a domain model to an API response DTO.
func ToEventResponse(e *model.Event) EventResponse {
	return EventResponse{
		ID:              e.ID.String(),
		Title:           e.Title,
		StartAt:         e.StartAt.Format(time.RFC3339),
		EndAt:           e.EndAt.Format(time.RFC3339),
		AllDay:          e.AllDay,
		Timezone:        e.Timezone,
		LocationName:    e.LocationName,
		LocationAddress: e.LocationAddress,
		LocationLat:     e.LocationLat,
		LocationLng:     e.LocationLng,
		ReminderOffsets: []int64(e.ReminderOffsets),
		Notes:           e.Notes,
		CreatedAt:       e.CreatedAt.Format(time.RFC3339),
		UpdatedAt:       e.UpdatedAt.Format(time.RFC3339),
	}
}

// ToEventListResponse converts a slice of domain models to an API list response.
func ToEventListResponse(events []model.Event) EventListResponse {
	responses := make([]EventResponse, len(events))
	for i := range events {
		responses[i] = ToEventResponse(&events[i])
	}
	return EventListResponse{
		Events: responses,
		Count:  len(responses),
	}
}
