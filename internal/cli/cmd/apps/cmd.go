package cmd_apps

import (
	"github.com/spf13/cobra"
)

// NewAppsCmd creates and returns the apps command with all subcommands.
func NewAppsCmd(cliName string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "apps",
		Short: "Manage apps",
		Long:  `Commands for managing apps on Shipable.`,
	}

	cmd.AddCommand(newListCmd(cliName))

	return cmd
}
