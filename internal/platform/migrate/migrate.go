// Package migrate 在启动时执行 GORM 与 SQL 版本化迁移。
package migrate

import (
	"embed"
	"fmt"
	"io/fs"
	"matrix/internal/platform/config"
	platformdb "matrix/internal/platform/db"
	"os"
	"path/filepath"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"gorm.io/gorm"
)

//go:embed *.sql
var embedSQL embed.FS

// Up 执行 GORM AutoMigrate 与 SQL 版本化迁移（若配置启用）。
func Up(db *gorm.DB, cfg config.DatabaseConfig) error {
	if cfg.AutoMigrate {
		if err := platformdb.AutoMigrate(db); err != nil {
			return err
		}
	}
	if !cfg.SQLMigrate {
		return nil
	}
	dir := migrationsDir()
	if info, err := os.Stat(dir); err == nil && info.IsDir() {
		abs, err := filepath.Abs(dir)
		if err != nil {
			return err
		}
		m, err := migrate.New("file://"+filepath.ToSlash(abs), cfg.DSN)
		if err != nil {
			return fmt.Errorf("migrate file source: %w", err)
		}
		if err := m.Up(); err != nil && err != migrate.ErrNoChange {
			return fmt.Errorf("migrate up: %w", err)
		}
		return nil
	}
	sub, err := fs.Sub(embedSQL, ".")
	if err != nil {
		return err
	}
	source, err := iofs.New(sub, ".")
	if err != nil {
		return err
	}
	m, err := migrate.NewWithSourceInstance("iofs", source, cfg.DSN)
	if err != nil {
		return err
	}
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return err
	}
	return nil
}

// migrationsDir 解析 SQL 迁移文件目录绝对路径。
func migrationsDir() string {
	if dir := os.Getenv("MATRIX_MIGRATIONS"); dir != "" {
		return dir
	}
	return "migrations"
}
