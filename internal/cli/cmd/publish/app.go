package cmd_publish

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/kula-app/ship/internal/cli/service"
)

// PublishAppCommandDeps declares the dependencies required by the publish app command.
type PublishAppCommandDeps = PublishCommandsDeps

// newAppCmd creates the "publish app" command.
func newAppCmd(cliName string, deps PublishAppCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "app",
		Short:   "Publish app binary only",
		Long:    `Trigger a publish limited to the app binary.`,
		Example: fmt.Sprintf(`  %[1]s publish app --app-id <uuid>`, cliName),
	}

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runPublishApp(cmd, deps, cliName)
	}

	return cmd
}

func runPublishApp(cmd *cobra.Command, deps PublishAppCommandDeps, cliName string) error {
	return runPublishJob(cmd, deps, fmt.Sprintf("%s publish app", cliName), "app",
		func(svc *service.ShipService, appID string, platforms []string) (*service.PublishJobResult, []byte, error) {
			return svc.TriggerPartialPublish(cmd.Context(), appID, "app", platforms)
		})
}
