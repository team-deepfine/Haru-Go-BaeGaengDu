package dto

// ParseVoiceRequest is the request body for voice text parsing.
type ParseVoiceRequest struct {
	Text string `json:"text" binding:"required"`
}

// ParseVoiceResponse is the response body for voice text parsing.
type ParseVoiceResponse struct {
	Event            CreateEventRequest `json:"event"`
	Confidence       float64            `json:"confidence"`
	FollowUpQuestion *string            `json:"followUpQuestion"`
}
