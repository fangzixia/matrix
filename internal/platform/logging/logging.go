// Package logging 初始化 slog 与按天归档的多类别日志。
package logging

import (
	"io"
	"log/slog"
	"matrix/internal/platform/config"
	"matrix/internal/platform/storage"
	"os"
	"strings"
)

// Loggers 是四类按天归档的日志实例。
type Loggers struct {
	System  *slog.Logger
	Access  *AccessLog
	closers []io.Closer
}

// Init 根据配置创建四类 Logger，写入按天归档的日志文件；开发模式下 system 同时输出到 stderr。
func Init(cfg config.LoggingConfig, paths storage.Paths, dev bool) (*Loggers, error) {
	if err := os.MkdirAll(paths.LogDir, 0o755); err != nil {
		return nil, err
	}
	retention := cfg.RetentionDays
	cats := paths.LogCategories

	systemW, err := openDailyWriter(paths.LogDir, cats.System, retention)
	if err != nil {
		return nil, err
	}
	apiW, err := openDailyWriter(paths.LogDir, cats.API, retention)
	if err != nil {
		_ = systemW.Close()
		return nil, err
	}
	llmW, err := openDailyWriter(paths.LogDir, cats.LLM, retention)
	if err != nil {
		_ = systemW.Close()
		_ = apiW.Close()
		return nil, err
	}
	agentW, err := openDailyWriter(paths.LogDir, cats.Agent, retention)
	if err != nil {
		_ = systemW.Close()
		_ = apiW.Close()
		_ = llmW.Close()
		return nil, err
	}

	var systemOut io.Writer = systemW
	if dev {
		systemOut = io.MultiWriter(os.Stderr, systemW)
	}

	level := parseLevel(cfg.Level)
	opts := &slog.HandlerOptions{Level: level}
	var h slog.Handler
	if strings.ToLower(cfg.Format) == "text" || dev {
		h = slog.NewTextHandler(systemOut, opts)
	} else {
		h = slog.NewJSONHandler(systemOut, opts)
	}
	systemLog := slog.New(h)
	slog.SetDefault(systemLog)

	llmLines = newJSONLineWriter(llmW)
	agentLines = newJSONLineWriter(agentW)

	loggers := &Loggers{
		System:  systemLog,
		Access:  &AccessLog{w: apiW},
		closers: []io.Closer{systemW, apiW, llmW, agentW},
	}
	return loggers, nil
}

// Close 关闭所有日志文件句柄。
func (l *Loggers) Close() error {
	if l == nil {
		return nil
	}
	var first error
	for _, c := range l.closers {
		if c == nil {
			continue
		}
		if err := c.Close(); err != nil && first == nil {
			first = err
		}
	}
	return first
}

func parseLevel(level string) slog.Level {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// Info 写入 system 日志。
func Info(msg string, args ...any) { slog.Info(msg, args...) }

// Warn 写入 system 日志。
func Warn(msg string, args ...any) { slog.Warn(msg, args...) }
