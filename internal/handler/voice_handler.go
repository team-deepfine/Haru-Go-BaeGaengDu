package handler

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/daewon/haru/internal/dto"
	"github.com/daewon/haru/internal/model"
	"github.com/daewon/haru/internal/service"
	"github.com/daewon/haru/pkg/response"
	"github.com/gin-gonic/gin"
)

// VoiceHandler handles HTTP requests for voice parsing.
type VoiceHandler struct {
	svc service.VoiceParsingService
}

// NewVoiceHandler creates a new VoiceHandler.
func NewVoiceHandler(svc service.VoiceParsingService) *VoiceHandler {
	return &VoiceHandler{svc: svc}
}

// RegisterRoutes registers voice parsing routes on a Gin router group.
func (h *VoiceHandler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.POST("/events/parse-voice", h.ParseVoice)
}

// ParseVoice handles POST /api/events/parse-voice.
func (h *VoiceHandler) ParseVoice(c *gin.Context) {
	var req dto.ParseVoiceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	if strings.TrimSpace(req.Text) == "" {
		response.Error(c, http.StatusBadRequest, model.ErrTextRequired.Error())
		return
	}

	result, err := h.svc.ParseVoice(c.Request.Context(), service.ParseVoiceInput{
		Text: req.Text,
	})
	if err != nil {
		handleVoiceServiceError(c, err)
		return
	}

	response.JSON(c, http.StatusOK, result)
}

func handleVoiceServiceError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, model.ErrTextRequired):
		response.Error(c, http.StatusBadRequest, err.Error())
	case errors.Is(err, model.ErrParsingFailed):
		response.Error(c, http.StatusUnprocessableEntity, err.Error())
	case errors.Is(err, model.ErrAIServiceUnavailable):
		response.Error(c, http.StatusBadGateway, err.Error())
	default:
		slog.Error("internal error in voice parsing", "error", err)
		response.Error(c, http.StatusInternalServerError, "internal server error")
	}
}
