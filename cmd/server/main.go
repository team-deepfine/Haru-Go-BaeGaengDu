package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/daewon/haru/config"
	"github.com/daewon/haru/internal/handler"
	"github.com/daewon/haru/internal/model"
	"github.com/daewon/haru/internal/repository"
	"github.com/daewon/haru/internal/router"
	"github.com/daewon/haru/internal/service"
	"github.com/daewon/haru/pkg/database"
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

	if err := db.AutoMigrate(&model.Event{}); err != nil {
		slog.Error("failed to run migrations", "error", err)
		os.Exit(1)
	}
	slog.Info("database migration completed")

	// Wire dependencies
	eventRepo := repository.NewEventRepository(db)
	eventSvc := service.NewEventService(eventRepo)
	eventHandler := handler.NewEventHandler(eventSvc)

	r := router.New(eventHandler)

	addr := fmt.Sprintf(":%s", cfg.Port)
	slog.Info("starting server", "addr", addr)
	if err := r.Run(addr); err != nil {
		slog.Error("server failed", "error", err)
		os.Exit(1)
	}
}
