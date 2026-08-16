package cmd_publish

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/kula-app/ship/internal/cli/service"
)

// PublishScreenshotsCommandDeps declares the dependencies required by the publish screenshots command.
type PublishScreenshotsCommandDeps = PublishCommandsDeps

// newScreenshotsCmd creates the "publish screenshots" command.
func newScreenshotsCmd(cliName string, deps PublishScreenshotsCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "screenshots",
		Short:   "Publish screenshots only",
		Long:    `Trigger a publish limited to the app's store screenshots.`,
		Example: fmt.Sprintf(`  %[1]s publish screenshots --app-id <uuid>`, cliName),
	}

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runPublishScreenshots(cmd, deps, cliName)
	}

	return cmd
}

func runPublishScreenshots(cmd *cobra.Command, deps PublishScreenshotsCommandDeps, cliName string) error {
	return runPublishJob(cmd, deps, fmt.Sprintf("%s publish screenshots", cliName), "screenshots",
		func(svc *service.ShipService, appID string, platforms []string) (*service.PublishJobResult, []byte, error) {
			return svc.TriggerPartialPublish(cmd.Context(), appID, "screenshots", platforms)
		})
}
