package service

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/daewon/haru/internal/dto"
	"github.com/daewon/haru/internal/model"
	"github.com/daewon/haru/pkg/gemini"
	"google.golang.org/genai"
)

const defaultPromptPath = "prompts/voice_parse.txt"

var createEventFuncDecl = &genai.FunctionDeclaration{
	Name:        "create_event",
	Description: "Extract structured calendar event information from the given text.",
	Parameters: &genai.Schema{
		Type:     genai.TypeObject,
		Required: []string{"title", "startAt", "endAt", "allDay", "confidence"},
		Properties: map[string]*genai.Schema{
			"title":            {Type: genai.TypeString, Description: "Event title"},
			"startAt":          {Type: genai.TypeString, Description: "Start time in ISO-8601 format with timezone offset"},
			"endAt":            {Type: genai.TypeString, Description: "End time in ISO-8601 format with timezone offset"},
			"allDay":           {Type: genai.TypeBoolean, Description: "Whether this is an all-day event"},
			"locationName":     {Type: genai.TypeString, Description: "Location name (nullable)"},
			"locationAddress":  {Type: genai.TypeString, Description: "Location address (nullable)"},
			"reminderOffsets":  {Type: genai.TypeArray, Items: &genai.Schema{Type: genai.TypeInteger}, Description: "Reminder offsets in minutes"},
			"notes":            {Type: genai.TypeString, Description: "Notes (nullable)"},
			"confidence":       {Type: genai.TypeNumber, Description: "Parsing confidence score (0.0-1.0)"},
			"followUpQuestion": {Type: genai.TypeString, Description: "Follow-up question when information is insufficient (nullable)"},
		},
	},
}

type voiceParsingService struct {
	geminiClient *gemini.Client
	timezone     string
	systemPrompt string
}

// NewVoiceParsingService creates a new VoiceParsingService.
// It loads the system prompt from the file at promptPath.
func NewVoiceParsingService(geminiClient *gemini.Client, timezone, promptPath string) (VoiceParsingService, error) {
	if promptPath == "" {
		promptPath = defaultPromptPath
	}

	data, err := os.ReadFile(promptPath)
	if err != nil {
		return nil, fmt.Errorf("load voice parse prompt from %s: %w", promptPath, err)
	}

	slog.Info("voice parse prompt loaded", "path", promptPath)

	return &voiceParsingService{
		geminiClient: geminiClient,
		timezone:     timezone,
		systemPrompt: string(data),
	}, nil
}

const llmTimeout = 30 * time.Second

func (s *voiceParsingService) ParseVoice(ctx context.Context, input ParseVoiceInput) (*dto.ParseVoiceResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, llmTimeout)
	defer cancel()

	loc, err := time.LoadLocation(s.timezone)
	if err != nil {
		return nil, fmt.Errorf("load timezone %s: %w", s.timezone, err)
	}

	now := time.Now().In(loc)
	userPrompt := fmt.Sprintf("Current time: %s %s\nTimezone: %s\n\nUser input: %s",
		now.Format("2006-01-02 Monday 15:04"), s.timezone, s.timezone, input.Text)

	var result *gemini.FunctionCallResult
	maxRetries := 3
	for attempt := range maxRetries {
		result, err = s.geminiClient.GenerateWithFunctionCall(ctx, s.systemPrompt, userPrompt, createEventFuncDecl)
		if err == nil {
			break
		}
		slog.Warn("gemini function call attempt failed", "attempt", attempt+1, "error", err)
		if attempt < maxRetries-1 {
			time.Sleep(5 * time.Second)
		}
	}
	if err != nil {
		return nil, fmt.Errorf("%w: %v", model.ErrAIServiceUnavailable, err)
	}

	return buildParseVoiceResponse(result.Args, s.timezone)
}

func buildParseVoiceResponse(args map[string]any, timezone string) (*dto.ParseVoiceResponse, error) {
	title, _ := args["title"].(string)
	if title == "" {
		return nil, model.ErrParsingFailed
	}

	startAt, _ := args["startAt"].(string)
	endAt, _ := args["endAt"].(string)
	if startAt == "" || endAt == "" {
		return nil, model.ErrParsingFailed
	}

	allDay, _ := args["allDay"].(bool)
	confidence, _ := args["confidence"].(float64)

	event := dto.CreateEventRequest{
		Title:    title,
		StartAt:  startAt,
		EndAt:    endAt,
		AllDay:   allDay,
		Timezone: timezone,
	}

	if v, ok := args["locationName"].(string); ok && v != "" {
		event.LocationName = &v
	}
	if v, ok := args["locationAddress"].(string); ok && v != "" {
		event.LocationAddress = &v
	}
	if offsets, ok := args["reminderOffsets"].([]any); ok {
		for _, o := range offsets {
			switch v := o.(type) {
			case float64:
				event.ReminderOffsets = append(event.ReminderOffsets, int64(v))
			}
		}
	}
	if v, ok := args["notes"].(string); ok && v != "" {
		event.Notes = &v
	}

	resp := &dto.ParseVoiceResponse{
		Event:      event,
		Confidence: confidence,
	}

	if v, ok := args["followUpQuestion"].(string); ok && v != "" {
		resp.FollowUpQuestion = &v
	}

	return resp, nil
}
