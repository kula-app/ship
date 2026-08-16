package cmd_auth

import (
	"fmt"

	"github.com/getsentry/sentry-go"
	"github.com/spf13/cobra"

	"github.com/kula-app/ship/internal/cli/bootstrap"
	"github.com/kula-app/ship/internal/cli/db"
	"github.com/kula-app/ship/internal/cli/helpers"
)

// AuthLogoutCommandDeps declares the dependencies required by the auth logout command.
type AuthLogoutCommandDeps interface {
	bootstrap.LoggerFactory
}

// newLogoutCmd creates the "auth logout" command.
func newLogoutCmd(cliName string, deps AuthLogoutCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "logout",
		Short:   "Log out of Shipable",
		Long:    `Remove locally stored authentication credentials.`,
		Example: fmt.Sprintf(`  %[1]s auth logout`, cliName),
	}

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runAuthLogout(cmd, deps, cliName)
	}

	return cmd
}

func runAuthLogout(cmd *cobra.Command, deps AuthLogoutCommandDeps, cliName string) error {
	// Start root Sentry transaction for CLI command
	transaction := helpers.StartCommandTransaction(cmd, fmt.Sprintf("%s auth logout", cliName))
	transaction.SetData("command", "auth.logout")
	transaction.SetData("cli_name", cliName)
	defer transaction.Finish()

	logger := deps.GetLogger()

	if !db.IsAuthenticated() {
		helpers.PrintMessage(cmd, "You are not currently authenticated.")
		transaction.Status = sentry.SpanStatusOK
		return nil
	}

	if err := db.ClearAuth(); err != nil {
		return helpers.FailInternal(transaction, fmt.Errorf("failed to clear credentials: %w", err))
	}

	logger.InfoContext(cmd.Context(), "credentials cleared")
	helpers.PrintMessage(cmd, "Successfully logged out.")

	transaction.Status = sentry.SpanStatusOK
	return nil
}
