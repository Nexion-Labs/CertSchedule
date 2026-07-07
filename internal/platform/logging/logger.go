// Package logging configures the application-wide structured logger.
package logging

import (
	"log/slog"
	"os"
)

// New builds a JSON slog.Logger writing to stdout at the given level.
func New(debug bool) *slog.Logger {
	level := slog.LevelInfo
	if debug {
		level = slog.LevelDebug
	}
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	logger := slog.New(handler)
	slog.SetDefault(logger)
	return logger
}
