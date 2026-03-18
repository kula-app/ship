package logging

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/lmittmann/tint"
	"github.com/mattn/go-isatty"
)

// MultiHandler sends log records to multiple handlers simultaneously.
// This allows us to log to both Sentry and terminal when Sentry is enabled,
// providing both error tracking and local development visibility.
type MultiHandler struct {
	handlers []slog.Handler
}

// NewMultiHandler creates a new MultiHandler that forwards log records to all provided handlers.
// Each handler will receive the same log record, allowing for different processing strategies
// (e.g., one for Sentry error tracking, one for terminal output).
func NewMultiHandler(handlers ...slog.Handler) *MultiHandler {
	return &MultiHandler{handlers: handlers}
}

// Enabled reports whether the handler handles records at the given level.
// It returns true if any of the underlying handlers handle the level.
// This ensures that log records are processed if at least one handler can handle them.
func (h *MultiHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, handler := range h.handlers {
		if handler.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

// Handle forwards the log record to all underlying handlers.
// If a handler fails, processing continues with the remaining handlers
// to ensure that logging failures don't cascade and break other outputs.
func (h *MultiHandler) Handle(ctx context.Context, record slog.Record) error {
	for _, handler := range h.handlers {
		if handler.Enabled(ctx, record.Level) {
			if err := handler.Handle(ctx, record); err != nil {
				// Continue processing other handlers even if one fails
				// This ensures terminal output continues even if Sentry fails
				continue
			}
		}
	}
	return nil
}

// WithAttrs returns a new MultiHandler whose handlers have the given attributes.
// This method ensures that attributes are propagated to all underlying handlers,
// maintaining consistency across different output destinations.
func (h *MultiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	newHandlers := make([]slog.Handler, len(h.handlers))
	for i, handler := range h.handlers {
		newHandlers[i] = handler.WithAttrs(attrs)
	}
	return NewMultiHandler(newHandlers...)
}

// WithGroup returns a new MultiHandler whose handlers have the given group name.
// This method ensures that grouping is applied consistently across all handlers,
// maintaining log structure regardless of output destination.
func (h *MultiHandler) WithGroup(name string) slog.Handler {
	newHandlers := make([]slog.Handler, len(h.handlers))
	for i, handler := range h.handlers {
		newHandlers[i] = handler.WithGroup(name)
	}
	return NewMultiHandler(newHandlers...)
}

// NewTerminalHandler creates a terminal handler with color support if output is a TTY.
// This provides consistent terminal logging whether Sentry is enabled or not.
//
// When output is directed to a terminal (TTY), it uses colorized output via tint
// for better readability during development. When output is redirected to files
// or pipes, it uses plain text for better compatibility with log aggregation tools.
func NewTerminalHandler() slog.Handler {
	if isatty.IsTerminal(os.Stderr.Fd()) {
		// Terminal output - use colors for better readability during development
		// Write to stderr to separate logs from standard structured output
		return tint.NewHandler(os.Stderr, &tint.Options{
			AddSource:  true,
			Level:      slog.LevelDebug,
			TimeFormat: time.DateTime,
		})
	}

	// Non-terminal output (logs, files, pipes) - use plain text for compatibility
	// This is important for CI/CD systems and log aggregation tools
	// Always write logs to stderr to keep stdout clean for structured output
	return slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})
}
