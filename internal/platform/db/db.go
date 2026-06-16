package db

import (
	"fmt"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"matrix/internal/platform/config"
	"matrix/internal/platform/db/models"
)

func Open(cfg config.DatabaseConfig) (*gorm.DB, error) {
	gcfg := &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Warn),
	}
	db, err := gorm.Open(postgres.Open(cfg.DSN), gcfg)
	if err != nil {
		return nil, fmt.Errorf("db: open: %w", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	if cfg.MaxOpenConns > 0 {
		sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	}
	sqlDB.SetConnMaxLifetime(time.Hour)
	return db, nil
}

func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(models.All()...)
}
