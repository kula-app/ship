package cmd_auth

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/kula-app/ship/internal/cli/db"
)

// newLogoutCmd creates the "auth logout" command.
func newLogoutCmd(cliName string) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "logout",
		Short:   "Log out of Shipable",
		Long:    `Remove locally stored authentication credentials.`,
		Example: fmt.Sprintf(`  %s auth logout`, cliName),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLogout(cmd, args, cliName)
		},
	}

	return cmd
}

func runLogout(_ *cobra.Command, _ []string, _ string) error {
	if !db.IsAuthenticated() {
		fmt.Fprintln(os.Stderr, "You are not currently authenticated.")
		return nil
	}

	if err := db.ClearAuth(); err != nil {
		return fmt.Errorf("failed to clear credentials: %w", err)
	}

	fmt.Fprintln(os.Stderr, "Successfully logged out.")
	return nil
}
