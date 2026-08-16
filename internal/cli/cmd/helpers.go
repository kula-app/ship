package cmd

import (
	"fmt"

	"github.com/getsentry/sentry-go"
	"github.com/spf13/cobra"

	"github.com/kula-app/ship/internal/cli/config"
	"github.com/kula-app/ship/internal/cli/service"
)

// outputJSON writes the raw API response body to stdout.
// This is shared across all commands that support JSON output; the server
// payload is passed through untouched so it stays machine-readable.
func outputJSON(cmd *cobra.Command, body []byte) error {
	_, err := fmt.Fprintln(cmd.OutOrStdout(), string(body))
	return err
}

// resolveLogFormat reads and validates the global --log-format flag.
// It returns true when the caller should emit JSON instead of a rendered table.
func resolveLogFormat(cmd *cobra.Command) (bool, error) {
	logFormat, err := cmd.Flags().GetString("log-format")
	if err != nil {
		return false, fmt.Errorf("failed to get log format: %w", err)
	}

	if logFormat != "text" && logFormat != "json" {
		return false, fmt.Errorf("invalid log format: %s (must be 'text' or 'json')", logFormat)
	}

	return logFormat == "json", nil
}

// resolveShipService resolves credentials and builds the client-bound service
// used by every command that talks to the API.
//
// The returned credentials carry the API URL, which commands need when they
// report the resolved endpoint without sending a request.
func resolveShipService(cmd *cobra.Command, deps ShipCommandDeps) (*service.ShipService, *config.Credentials, error) {
	credentials, err := config.ResolveCredentials(cmd)
	if err != nil {
		return nil, nil, err
	}

	return deps.GetShipService(deps.GetLogger(), credentials), credentials, nil
}

// failInvalidArgument marks the transaction as a user error and returns the error.
// Commands use it for bad flags and other input the user controls, keeping those
// events out of Sentry via the error classifier.
func failInvalidArgument(transaction *sentry.Span, err error) error {
	transaction.Status = sentry.SpanStatusInvalidArgument
	return err
}

// failUnauthenticated marks the transaction as an authentication failure.
func failUnauthenticated(transaction *sentry.Span, err error) error {
	transaction.Status = sentry.SpanStatusUnauthenticated
	return err
}

// failInternal marks the transaction as an application error, which the
// classifier reports to Sentry.
func failInternal(transaction *sentry.Span, err error) error {
	transaction.Status = sentry.SpanStatusInternalError
	return err
}

// printMessage writes a human-facing diagnostic to stderr, keeping stdout
// reserved for structured output.
func printMessage(cmd *cobra.Command, format string, args ...any) {
	fmt.Fprintf(cmd.ErrOrStderr(), format+"\n", args...)
}
