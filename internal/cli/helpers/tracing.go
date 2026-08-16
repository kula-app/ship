package helpers

import (
	"os"

	"github.com/getsentry/sentry-go"
	"github.com/spf13/cobra"
)

// StartCommandTransaction creates a Sentry transaction with proper distributed trace
// continuation from --sentry-trace/--sentry-baggage flags.
func StartCommandTransaction(cmd *cobra.Command, name string) *sentry.Span {
	hub := sentry.CurrentHub()
	sentryTrace, _ := cmd.Flags().GetString("sentry-trace")
	if sentryTrace == "" {
		sentryTrace = os.Getenv("SENTRY_TRACE")
	}
	sentryBaggage, _ := cmd.Flags().GetString("sentry-baggage")
	if sentryBaggage == "" {
		sentryBaggage = os.Getenv("SENTRY_BAGGAGE")
	}
	transaction := sentry.StartTransaction(
		sentry.SetHubOnContext(cmd.Context(), hub),
		name,
		sentry.WithOpName("console.command"),
		sentry.ContinueTrace(hub, sentryTrace, sentryBaggage),
	)
	cmd.SetContext(transaction.Context())
	return transaction
}
