package cmd_publish

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/kula-app/ship/internal/cli/config"
)

func newValidateCmd(cliName string) *cobra.Command {
	return &cobra.Command{
		Use:     "validate",
		Short:   "Pre-publish validation",
		Long:    `Generates the Xcode project to verify the app configuration is valid before publishing.`,
		Example: fmt.Sprintf(`  %s publish validate --app-id <uuid>`, cliName),
		RunE: func(c *cobra.Command, args []string) error {
			return runValidate(c)
		},
	}
}

func runValidate(c *cobra.Command) error {
	appID, _ := c.Flags().GetString("app-id")

	client, err := config.AuthenticatedClient(c.Root().Name())
	if err != nil {
		return err
	}

	platforms, _ := c.Flags().GetStringSlice("platform")
	body, err := client.Post(
		fmt.Sprintf("/api/app/%s/pre-publish/generate", appID),
		publishJobRequest{Platforms: platforms},
	)
	if err != nil {
		return fmt.Errorf("failed to trigger validation: %w", err)
	}

	return printJobResponse(c, body)
}
