package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/getsentry/sentry-go"
	sentryslog "github.com/getsentry/sentry-go/slog"

	"github.com/kula-app/ship/internal/cli/cmd"
	"github.com/kula-app/ship/internal/logging"
)

// BuildMetadata holds build-time information injected via ldflags.
type BuildMetadata struct {
	Version     string
	Commit      string
	Date        string
	Release     string
	Environment string
}

// run is the main application logic, separated from main() for testability.
func run(ctx context.Context, _ []string, getenv func(key string) string, _ *os.File, metadata BuildMetadata) error {
	// Allow disabling Sentry via environment variable
	sentryEnabled := getenv("TELEMETRY_ENABLED")
	if sentryEnabled != "false" {
		err := sentry.Init(sentry.ClientOptions{
			Dsn:                  "https://1164207c04ce7534ca539fe8897f0d02@o997061.ingest.us.sentry.io/4511062165028864",
			Debug:                false,
			Environment:          metadata.Environment,
			Release:              metadata.Release,
			AttachStacktrace:     true,
			SendDefaultPII:       true,
			SampleRate:           1.0,
			EnableLogs:           true,
			EnableTracing:        true,
			TracesSampleRate:     1.0,
			PropagateTraceparent: true,
			BeforeSend: func(event *sentry.Event, hint *sentry.EventHint) *sentry.Event {
				if event.Type == "transaction" {
					return event
				}

				if metadata.Date != "" {
					if event.Tags == nil {
						event.Tags = make(map[string]string)
					}
					event.Tags["build_date"] = metadata.Date
				}

				return event
			},
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "sentry.Init: %s\n", err)
		}

		sentryHandler := sentryslog.Option{
			EventLevel: []slog.Level{},
			LogLevel:   []slog.Level{slog.LevelError, slog.LevelWarn, slog.LevelInfo},
		}.NewSentryHandler(ctx)

		terminalHandler := logging.NewTerminalHandler()
		multiHandler := logging.NewMultiHandler(sentryHandler, terminalHandler)
		logger := slog.New(multiHandler)
		slog.SetDefault(logger)

		defer sentry.Flush(2 * time.Second)
	}

	rootCmd := cmd.NewRootCommand("ship", cmd.BuildMetadata{
		Version: metadata.Version,
		Commit:  metadata.Commit,
		Date:    metadata.Date,
	})
	if err := rootCmd.Execute(); err != nil {
		sentry.CaptureException(err)
		sentry.Flush(2 * time.Second)

		return err
	}

	return nil
}
