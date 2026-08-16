package cmd_publish

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/kula-app/ship/internal/cli/service"
)

// PublishValidateCommandDeps declares the dependencies required by the publish validate command.
type PublishValidateCommandDeps = PublishCommandsDeps

// newValidateCmd creates the "publish validate" command.
func newValidateCmd(cliName string, deps PublishValidateCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "validate",
		Short:   "Pre-publish validation",
		Long:    `Generates the Xcode project to verify the app configuration is valid before publishing.`,
		Example: fmt.Sprintf(`  %[1]s publish validate --app-id <uuid>`, cliName),
	}

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runPublishValidate(cmd, deps, cliName)
	}

	return cmd
}

func runPublishValidate(cmd *cobra.Command, deps PublishValidateCommandDeps, cliName string) error {
	return runPublishJob(cmd, deps, fmt.Sprintf("%s publish validate", cliName), "validate",
		func(svc *service.ShipService, appID string, platforms []string) (*service.PublishJobResult, []byte, error) {
			return svc.TriggerValidation(cmd.Context(), appID, platforms)
		})
}
