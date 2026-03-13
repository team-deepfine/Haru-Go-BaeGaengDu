package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/daewon/haru/config"
	"github.com/daewon/haru/internal/dto"
	"github.com/daewon/haru/internal/handler"
	"github.com/daewon/haru/internal/model"
	"github.com/daewon/haru/internal/repository"
	"github.com/daewon/haru/internal/router"
	"github.com/daewon/haru/internal/service"
	"github.com/daewon/haru/pkg/database"
	"github.com/daewon/haru/pkg/fcm"
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

	if err := db.AutoMigrate(
		&model.User{}, &model.RefreshToken{}, &model.Event{},
		&model.Notification{}, &model.DeviceToken{},
	); err != nil {
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

	// Initialize OAuth providers
	appleClient, err := oauth.NewAppleClient(
		cfg.Apple.ClientID, cfg.Apple.TeamID, cfg.Apple.KeyID,
		cfg.Apple.PrivateKey, cfg.Apple.RedirectURI,
	)
	if err != nil {
		slog.Error("failed to create apple client", "error", err)
		os.Exit(1)
	}
	kakaoClient := oauth.NewKakaoClient(cfg.Kakao.ClientID, cfg.Kakao.ClientSecret, cfg.Kakao.RedirectURI)

	// Wire auth dependencies
	userRepo := repository.NewUserRepository(db)
	tokenRepo := repository.NewTokenRepository(db)
	authSvc := service.NewAuthService(userRepo, tokenRepo, jwtManager, appleClient, kakaoClient)
	authHandler := handler.NewAuthHandler(authSvc)

	// Wire notification dependencies
	notifRepo := repository.NewNotificationRepository(db)
	deviceTokenRepo := repository.NewDeviceTokenRepository(db)
	notifScheduler := service.NewNotificationScheduler(notifRepo)

	// Wire event dependencies (with notification scheduler)
	eventRepo := repository.NewEventRepository(db)
	eventSvc := service.NewEventService(eventRepo, service.WithNotificationScheduler(notifScheduler))
	eventHandler := handler.NewEventHandler(eventSvc)

	// Wire device token dependencies
	deviceTokenSvc := service.NewDeviceTokenService(deviceTokenRepo)
	deviceTokenHandler := handler.NewDeviceTokenHandler(deviceTokenSvc)

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

	r := router.New(jwtManager, authHandler, eventHandler, voiceHandler, deviceTokenHandler)

	// Start notification worker if FCM is enabled
	var workerCancel context.CancelFunc
	if cfg.FCM.Enabled {
		fcmClient, err := fcm.NewClient(context.Background(), []byte(cfg.FCM.CredentialsJSON))
		if err != nil {
			slog.Error("failed to create FCM client", "error", err)
			os.Exit(1)
		}

		worker := service.NewNotificationWorker(notifRepo, deviceTokenRepo, fcmClient)
		workerCtx, cancel := context.WithCancel(context.Background())
		workerCancel = cancel
		go worker.Start(workerCtx)
		slog.Info("notification worker enabled")
	} else {
		slog.Warn("FCM_ENABLED not set, notification worker disabled")
	}

	// Start HTTP server with graceful shutdown
	addr := fmt.Sprintf(":%s", cfg.Port)
	srv := &http.Server{
		Addr:    addr,
		Handler: r,
	}

	go func() {
		slog.Info("starting server", "addr", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server failed", "error", err)
			os.Exit(1)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	slog.Info("shutting down server...")

	// Stop notification worker
	if workerCancel != nil {
		workerCancel()
	}

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("server forced to shutdown", "error", err)
	}

	slog.Info("server exited")
}

// noopVoiceParsingService returns an error when GEMINI_API_KEY is not configured.
type noopVoiceParsingService struct{}

func (s *noopVoiceParsingService) ParseVoice(_ context.Context, _ service.ParseVoiceInput) (*dto.ParseVoiceResponse, error) {
	return nil, model.ErrAIServiceUnavailable
}
