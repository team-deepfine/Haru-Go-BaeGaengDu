package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/daewon/haru/config"
	"github.com/daewon/haru/internal/dto"
	"github.com/daewon/haru/internal/handler"
	"github.com/daewon/haru/internal/model"
	"github.com/daewon/haru/internal/repository"
	"github.com/daewon/haru/internal/router"
	"github.com/daewon/haru/internal/service"
	"github.com/daewon/haru/pkg/database"
	"github.com/daewon/haru/pkg/gemini"
	jwtpkg "github.com/daewon/haru/pkg/jwt"
	"github.com/daewon/haru/pkg/oauth"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})))

	cfg := config.Load()

	db, err := database.New(cfg)
	if err != nil {
		slog.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}

	if err := db.AutoMigrate(&model.User{}, &model.RefreshToken{}, &model.Event{}); err != nil {
		slog.Error("failed to run migrations", "error", err)
		os.Exit(1)
	}
	slog.Info("database migration completed")

	// Validate required secrets
	if cfg.JWT.Secret == "" {
		slog.Error("JWT_SECRET is required")
		os.Exit(1)
	}

	// Parse JWT expiry durations
	accessExpiry, err := time.ParseDuration(cfg.JWT.AccessExpiry)
	if err != nil {
		slog.Error("invalid JWT_ACCESS_EXPIRY", "error", err)
		os.Exit(1)
	}
	refreshExpiry, err := time.ParseDuration(cfg.JWT.RefreshExpiry)
	if err != nil {
		slog.Error("invalid JWT_REFRESH_EXPIRY", "error", err)
		os.Exit(1)
	}

	// Initialize JWT manager
	jwtManager := jwtpkg.NewManager(cfg.JWT.Secret, accessExpiry, refreshExpiry)

	// Initialize Apple verifier
	appleVerifier := oauth.NewAppleVerifier(cfg.Apple.ClientID)

	// Wire auth dependencies
	userRepo := repository.NewUserRepository(db)
	tokenRepo := repository.NewTokenRepository(db)
	authSvc := service.NewAuthService(userRepo, tokenRepo, jwtManager, appleVerifier)
	authHandler := handler.NewAuthHandler(authSvc)

	// Wire event dependencies
	eventRepo := repository.NewEventRepository(db)
	eventSvc := service.NewEventService(eventRepo)
	eventHandler := handler.NewEventHandler(eventSvc)

	// Wire voice parsing dependencies
	var voiceSvc service.VoiceParsingService
	if cfg.Gemini.APIKey != "" {
		geminiClient, err := gemini.NewClient(context.Background(), cfg.Gemini.APIKey, cfg.Gemini.Model)
		if err != nil {
			slog.Error("failed to create gemini client", "error", err)
			os.Exit(1)
		}
		voiceSvc, err = service.NewVoiceParsingService(geminiClient, cfg.Gemini.Timezone, cfg.Gemini.PromptPath)
		if err != nil {
			slog.Error("failed to create voice parsing service", "error", err)
			os.Exit(1)
		}
	} else {
		slog.Warn("GEMINI_API_KEY not set, voice parsing endpoint will return 502")
		voiceSvc = &noopVoiceParsingService{}
	}
	voiceHandler := handler.NewVoiceHandler(voiceSvc)

	r := router.New(jwtManager, authHandler, eventHandler, voiceHandler)

	addr := fmt.Sprintf(":%s", cfg.Port)
	slog.Info("starting server", "addr", addr)
	if err := r.Run(addr); err != nil {
		slog.Error("server failed", "error", err)
		os.Exit(1)
	}
}

// noopVoiceParsingService returns an error when GEMINI_API_KEY is not configured.
type noopVoiceParsingService struct{}

func (s *noopVoiceParsingService) ParseVoice(_ context.Context, _ service.ParseVoiceInput) (*dto.ParseVoiceResponse, error) {
	return nil, model.ErrAIServiceUnavailable
}
