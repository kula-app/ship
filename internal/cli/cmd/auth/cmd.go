package cmd_auth

import (
	"github.com/spf13/cobra"
)

// NewAuthCmd creates and returns the auth command with all subcommands.
func NewAuthCmd(cliName string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Manage authentication",
		Long:  `Commands for managing authentication credentials for the Shipable API.`,
	}

	cmd.AddCommand(newLoginCmd(cliName))
	cmd.AddCommand(newLogoutCmd(cliName))

	return cmd
}
