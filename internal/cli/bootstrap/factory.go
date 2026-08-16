package bootstrap

import "log/slog"

// LoggerFactory provides a logger to CLI commands.
type LoggerFactory interface {
	GetLogger() *slog.Logger
}
