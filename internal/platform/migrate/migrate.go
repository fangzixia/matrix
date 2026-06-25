// Package migrate 在启动时执行 GORM 与 SQL 版本化迁移。
package migrate

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"

	"matrix/internal/platform/config"
	platformdb "matrix/internal/platform/db"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
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
		return fmt.Errorf("migrate embed source: %w", err)
	}
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("migrate up: %w", err)
	}
	return nil
}
