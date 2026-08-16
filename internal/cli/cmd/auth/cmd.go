package cmd_auth

import (
	"github.com/spf13/cobra"

	"github.com/kula-app/ship/internal/cli/bootstrap"
)

// AuthCommandsDeps declares the dependencies shared by the auth subcommands.
// They manage local credentials rather than calling the API, so a logger is all
// they need.
type AuthCommandsDeps interface {
	bootstrap.LoggerFactory
}

// NewAuthCmd creates and returns the auth command with all subcommands.
func NewAuthCmd(cliName string, deps AuthCommandsDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Manage authentication",
		Long:  `Commands for managing authentication credentials for the Shipable API.`,
	}

	cmd.AddCommand(newLoginCmd(cliName, deps))
	cmd.AddCommand(newLogoutCmd(cliName, deps))

	return cmd
}
