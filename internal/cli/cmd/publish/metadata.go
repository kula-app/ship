package cmd_publish

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/kula-app/ship/internal/cli/service"
)

// PublishMetadataCommandDeps declares the dependencies required by the publish metadata command.
type PublishMetadataCommandDeps = PublishCommandsDeps

// newMetadataCmd creates the "publish metadata" command.
func newMetadataCmd(cliName string, deps PublishMetadataCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "metadata",
		Short:   "Publish metadata only",
		Long:    `Trigger a publish limited to the app's store metadata.`,
		Example: fmt.Sprintf(`  %[1]s publish metadata --app-id <uuid>`, cliName),
	}

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runPublishMetadata(cmd, deps, cliName)
	}

	return cmd
}

func runPublishMetadata(cmd *cobra.Command, deps PublishMetadataCommandDeps, cliName string) error {
	return runPublishJob(cmd, deps, fmt.Sprintf("%s publish metadata", cliName), "metadata",
		func(svc *service.ShipService, appID string, platforms []string) (*service.PublishJobResult, []byte, error) {
			return svc.TriggerPartialPublish(cmd.Context(), appID, "metadata", platforms)
		})
}
