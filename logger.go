package main

import (
	"log/slog"
	"os"
)

func InitLogger(level string, filePath string) *slog.Logger {
	if filePath == "" {
		filePath = "ha-tray.log"
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
