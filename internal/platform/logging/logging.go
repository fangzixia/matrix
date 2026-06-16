package logging

import (
	"io"

	"log/slog"

	"os"

	"strings"

	"matrix/internal/platform/config"

	"matrix/internal/platform/storage"
)

func Init(cfg config.LoggingConfig, paths storage.Paths, dev bool) (*slog.Logger, error) {

	if err := os.MkdirAll(paths.LogDir, 0o755); err != nil {

		return nil, err

	}

	f, err := openLogWriter(paths.LogFile, cfg.MaxSizeMB, cfg.MaxBackups)

	if err != nil {

		return nil, err

	}

	var w io.Writer = f

	if dev {

		w = io.MultiWriter(os.Stderr, f)

	}

	level := slog.LevelInfo

	switch strings.ToLower(cfg.Level) {

	case "debug":

		level = slog.LevelDebug

	case "warn":

		level = slog.LevelWarn

	case "error":

		level = slog.LevelError

	}

	opts := &slog.HandlerOptions{Level: level}

	var h slog.Handler

	if strings.ToLower(cfg.Format) == "text" || dev {

		h = slog.NewTextHandler(w, opts)

	} else {

		h = slog.NewJSONHandler(w, opts)

	}

	log := slog.New(h)

	slog.SetDefault(log)

	return log, nil

}

// Legacy helpers for ai kernel packages during migration.

func Info(msg string, args ...any) { slog.Info(msg, args...) }

func Warn(msg string, args ...any) { slog.Warn(msg, args...) }

func Error(msg string, args ...any) { slog.Error(msg, args...) }

func Debug(msg string, args ...any) { slog.Debug(msg, args...) }
