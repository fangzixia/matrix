// Package db 打开 PostgreSQL 连接并执行 GORM 自动迁移。
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

// Open 打开 PostgreSQL 连接并配置连接池。
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

// AutoMigrate 对 models.All 注册的全部实体执行 GORM 自动建表。
func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(models.All()...)
}
