package logger

import (
	"log/slog"
	"os"
	"strings"

	"github.com/loomi-labs/clockkeeper/internal/env"
)

// Setup initializes the global slog logger based on environment configuration.
func Setup() {
	levelStr := env.GetString("LOG_LEVEL", "info")
	format := env.GetString("LOG_FORMAT", "text")

	var level slog.Level
	switch strings.ToLower(levelStr) {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{Level: level}

	var handler slog.Handler
	if strings.ToLower(format) == "json" {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	slog.SetDefault(slog.New(handler))
}
