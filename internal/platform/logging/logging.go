// Package logging 初始化 slog 日志输出，并提供 AI 内核迁移期的兼容辅助函数。
package logging

import (
	"io"
	"log/slog"
	"matrix/internal/platform/config"
	"matrix/internal/platform/storage"
	"os"
	"strings"
)

// Init 根据配置创建 slog Logger，写入日志文件并在开发模式下同时输出到 stderr。
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

// Info 写入 info 级别日志（兼容 AI 内核旧调用）。
func Info(msg string, args ...any) { slog.Info(msg, args...) }

// Warn 写入 warn 级别日志（兼容 AI 内核旧调用）。
func Warn(msg string, args ...any) { slog.Warn(msg, args...) }

// Error 写入 error 级别日志（兼容 AI 内核旧调用）。
func Error(msg string, args ...any) { slog.Error(msg, args...) }

// Debug 写入 debug 级别日志（兼容 AI 内核旧调用）。
func Debug(msg string, args ...any) { slog.Debug(msg, args...) }
