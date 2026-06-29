package logging

import "log/slog"

func logWriteFallback(msg string, err error) {
	if err == nil {
		slog.Warn(msg)
		return
	}
	slog.Warn(msg, "error", err)
}
