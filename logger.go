package main

import (
	"log/slog"
	"os"
	"path/filepath"
)

func InitLogger(level string, filePath string) *slog.Logger {
	filePath = logPath(filePath)

	if dir := filepath.Dir(filePath); dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			slog.Error("failed to create log dir, falling back to stderr", "error", err, "path", dir)
			return slog.Default()
		}
	}

	f, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		slog.Error("failed to open log file, falling back to stderr", "error", err, "path", filePath)
		return slog.Default()
	}

	var slogLevel slog.Level
	switch lower(level) {
	case "debug":
		slogLevel = slog.LevelDebug
	case "warn", "warning":
		slogLevel = slog.LevelWarn
	case "error":
		slogLevel = slog.LevelError
	default:
		slogLevel = slog.LevelInfo
	}

	handler := slog.NewJSONHandler(f, &slog.HandlerOptions{Level: slogLevel})
	logger := slog.New(handler)
	slog.SetDefault(logger)
	return logger
}
