package helpers

import (
	"fmt"

	"github.com/getsentry/sentry-go"
	"github.com/spf13/cobra"

	"github.com/kula-app/ship/internal/cli/bootstrap"
	"github.com/kula-app/ship/internal/cli/config"
	"github.com/kula-app/ship/internal/cli/service"
)

// ShipServiceProvider is the slice of the dependency container needed to build
// a client-bound service. Command deps interfaces satisfy it structurally.
type ShipServiceProvider interface {
	bootstrap.LoggerFactory
	service.ShipServiceFactory
}

// OutputJSON writes a raw API response body to stdout.
// This is shared across all commands that support JSON output; the server
// payload is passed through untouched so it stays machine-readable.
func OutputJSON(cmd *cobra.Command, body []byte) error {
	_, err := fmt.Fprintln(cmd.OutOrStdout(), string(body))
	return err
}

// ResolveLogFormat reads and validates the global --log-format flag.
// It returns true when the caller should emit JSON instead of rendered text.
func ResolveLogFormat(cmd *cobra.Command) (bool, error) {
	logFormat, err := cmd.Flags().GetString("log-format")
	if err != nil {
		return false, fmt.Errorf("failed to get log format: %w", err)
	}

	if logFormat != "text" && logFormat != "json" {
		return false, fmt.Errorf("invalid log format: %s (must be 'text' or 'json')", logFormat)
	}

	return logFormat == "json", nil
}

// ResolveShipService resolves credentials and builds the client-bound service
// used by every command that talks to the API.
//
// The returned credentials carry the API URL, which commands record on their
// transaction and report when no request is sent.
func ResolveShipService(cmd *cobra.Command, deps ShipServiceProvider) (*service.ShipService, *config.Credentials, error) {
	credentials, err := config.ResolveCredentials(cmd)
	if err != nil {
		return nil, nil, err
	}

	return deps.GetShipService(deps.GetLogger(), credentials), credentials, nil
}

// FailInvalidArgument marks the transaction as a user error and returns the error.
// Commands use it for bad flags and other input the user controls, keeping those
// events out of Sentry via the error classifier.
func FailInvalidArgument(transaction *sentry.Span, err error) error {
	transaction.Status = sentry.SpanStatusInvalidArgument
	return err
}

// FailUnauthenticated marks the transaction as an authentication failure.
func FailUnauthenticated(transaction *sentry.Span, err error) error {
	transaction.Status = sentry.SpanStatusUnauthenticated
	return err
}

// FailInternal marks the transaction as an application error, which the
// classifier reports to Sentry.
func FailInternal(transaction *sentry.Span, err error) error {
	transaction.Status = sentry.SpanStatusInternalError
	return err
}

// PrintMessage writes a human-facing diagnostic to stderr, keeping stdout
// reserved for structured output.
func PrintMessage(cmd *cobra.Command, format string, args ...any) {
	fmt.Fprintf(cmd.ErrOrStderr(), format+"\n", args...)
}
