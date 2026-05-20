// Package logger 提供统一的结构化日志；开发模式同时写入 stderr 与日志文件。
package logger

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

var (
	initOnce sync.Once
	initErr  error
)

// Init 初始化全局 slog 默认 Logger：始终写入用户配置目录下的 matrix.log；
// 开发模式下额外写入 stderr（双写）。
func Init() error {
	initOnce.Do(func() {
		initErr = initLogger()
	})
	return initErr
}

func initLogger() error {
	dev := isDev()

	logDir, logFile, err := logFilePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return fmt.Errorf("logger: mkdir %s: %w", logDir, err)
	}

	f, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("logger: open %s: %w", logFile, err)
	}

	var w io.Writer = f
	if dev {
		w = io.MultiWriter(os.Stderr, f)
	}

	level := slog.LevelInfo
	if dev {
		level = slog.LevelDebug
	}

	opts := &slog.HandlerOptions{Level: level}
	if dev {
		opts.AddSource = true
	}

	h := slog.NewTextHandler(w, opts)
	slog.SetDefault(slog.New(h))

	slog.Info("logger: initialized",
		"dev", dev,
		"file", logFile,
		"dual_write", dev,
	)
	return nil
}

func logFilePath() (dir, file string, err error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", "", fmt.Errorf("logger: user config dir: %w", err)
	}
	dir = filepath.Join(base, "matrix", "logs")
	file = filepath.Join(dir, "matrix.log")
	return dir, file, nil
}

// isDev 判定是否为开发模式（stderr 双写）。
// 触发条件（任一）：环境变量 MATRIX_DEV=1、编译标签 matrixdev、
// 可执行文件名含 -dev（wails dev 默认输出 matrix-dev）。
func isDev() bool {
	if os.Getenv("MATRIX_DEV") == "1" {
		return true
	}
	if buildTagDev {
		return true
	}
	exe, err := os.Executable()
	if err != nil {
		return false
	}
	base := strings.ToLower(filepath.Base(exe))
	return strings.Contains(base, "-dev") || strings.Contains(base, "_dev")
}

// Info 记录 Info 级别日志。
func Info(msg string, args ...any) {
	slog.Info(msg, args...)
}

// Debug 记录 Debug 级别日志。
func Debug(msg string, args ...any) {
	slog.Debug(msg, args...)
}

// Warn 记录 Warn 级别日志。
func Warn(msg string, args ...any) {
	slog.Warn(msg, args...)
}

// Error 记录 Error 级别日志。
func Error(msg string, args ...any) {
	slog.Error(msg, args...)
}

// Infof 以格式化消息记录 Info 级别日志（便于迁移原 log.Printf 调用）。
func Infof(format string, args ...any) {
	slog.Info(fmt.Sprintf(format, args...))
}

// Warnf 以格式化消息记录 Warn 级别日志。
func Warnf(format string, args ...any) {
	slog.Warn(fmt.Sprintf(format, args...))
}

// Errorf 以格式化消息记录 Error 级别日志。
func Errorf(format string, args ...any) {
	slog.Error(fmt.Sprintf(format, args...))
}

// Fatalf 记录 Error 并退出进程。
func Fatalf(format string, args ...any) {
	slog.Error(fmt.Sprintf(format, args...))
	os.Exit(1)
}
