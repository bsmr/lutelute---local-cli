// Package logging configures structured logging via log/slog.
package logging

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
)

// Setup configures the global slog logger.
//
//   - level: minimum log level (debug, info, error)
//   - logFile: optional path for JSON log file ("" = no file)
//
// Terminal output uses TextHandler (human-readable, stderr).
// File output uses JSONHandler (machine-parseable).
func Setup(level, logFile string) error {
	lvl := parseLevel(level)
	termHandler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: lvl})

	if logFile == "" {
		slog.SetDefault(slog.New(termHandler))
		return nil
	}

	f, err := os.OpenFile(logFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("cannot open log file %s: %w", logFile, err)
	}

	fileHandler := slog.NewJSONHandler(f, &slog.HandlerOptions{Level: lvl})
	multi := &multiHandler{handlers: []slog.Handler{termHandler, fileHandler}}
	slog.SetDefault(slog.New(multi))
	return nil
}

// parseLevel converts a string level name to slog.Level.
// Defaults to INFO for unrecognized values.
func parseLevel(s string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return slog.LevelDebug
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// ParseLevel is the exported version for use in tests.
func ParseLevel(s string) slog.Level { return parseLevel(s) }

// multiHandler fans out log records to multiple handlers.
type multiHandler struct {
	handlers []slog.Handler
}

func (h *multiHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, handler := range h.handlers {
		if handler.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

func (h *multiHandler) Handle(ctx context.Context, r slog.Record) error {
	for _, handler := range h.handlers {
		if !handler.Enabled(ctx, r.Level) {
			continue
		}
		if err := handler.Handle(ctx, r); err != nil {
			return err
		}
	}
	return nil
}

func (h *multiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	handlers := make([]slog.Handler, len(h.handlers))
	for i, handler := range h.handlers {
		handlers[i] = handler.WithAttrs(attrs)
	}
	return &multiHandler{handlers: handlers}
}

func (h *multiHandler) WithGroup(name string) slog.Handler {
	handlers := make([]slog.Handler, len(h.handlers))
	for i, handler := range h.handlers {
		handlers[i] = handler.WithGroup(name)
	}
	return &multiHandler{handlers: handlers}
}
