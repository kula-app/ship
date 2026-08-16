package cmd_apps

import (
	"github.com/spf13/cobra"

	"github.com/kula-app/ship/internal/cli/bootstrap"
	"github.com/kula-app/ship/internal/cli/service"
)

// AppsCommandsDeps declares the dependencies shared by the apps subcommands.
type AppsCommandsDeps interface {
	bootstrap.LoggerFactory
	service.ShipServiceFactory
}

// NewAppsCmd creates and returns the apps command with all subcommands.
func NewAppsCmd(cliName string, deps AppsCommandsDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "apps",
		Short: "Manage apps",
		Long:  `Commands for managing apps on Shipable.`,
	}

	cmd.AddCommand(newListCmd(cliName, deps))

	return cmd
}
