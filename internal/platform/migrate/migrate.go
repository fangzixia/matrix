package migrate

import (
	"embed"

	"fmt"

	"io/fs"

	"os"

	"path/filepath"

	"github.com/golang-migrate/migrate/v4"

	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/golang-migrate/migrate/v4/source/iofs"

	"gorm.io/gorm"

	"matrix/internal/platform/config"

	platformdb "matrix/internal/platform/db"
)

//go:embed *.sql

var embedSQL embed.FS

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

func migrationsDir() string {

	if dir := os.Getenv("MATRIX_MIGRATIONS"); dir != "" {

		return dir

	}

	return "migrations"

}
