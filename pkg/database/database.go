package database

import (
	"fmt"
	"log/slog"

	"github.com/daewon/haru/config"
	"gorm.io/driver/postgres"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// New creates a new GORM database connection based on configuration.
func New(cfg *config.Config) (*gorm.DB, error) {
	var dialector gorm.Dialector

	switch cfg.DB.Driver {
	case "postgres":
		if cfg.DB.URL == "" {
			return nil, fmt.Errorf("DATABASE_URL is required for postgres driver")
		}
		dialector = postgres.Open(cfg.DB.URL)
		slog.Info("using PostgreSQL database")
	case "sqlite":
		dialector = sqlite.Open("haru.db")
		slog.Info("using SQLite database (development mode)")
	default:
		return nil, fmt.Errorf("unsupported DB_DRIVER: %s", cfg.DB.Driver)
	}

	db, err := gorm.Open(dialector, &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	return db, nil
}
