// Package logger initializes the global structured logger for OmniLedger.
package logger

import (
	"log/slog"
	"os"
)

// Log is the global structured logger instance.
var Log *slog.Logger

// Init configures the global logger. Development uses human-readable text
// output; all other environments use JSON for machine parsing.
func Init(env string) {
	var handler slog.Handler
	opts := &slog.HandlerOptions{Level: slog.LevelInfo}
	if env == "development" {
		handler = slog.NewTextHandler(os.Stdout, opts)
	} else {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	}
	Log = slog.New(handler)
	slog.SetDefault(Log)
}
