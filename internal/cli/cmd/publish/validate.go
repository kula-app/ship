package cmd_publish

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/kula-app/ship/internal/cli/config"
)

func newValidateCmd(cliName string) *cobra.Command {
	return &cobra.Command{
		Use:   "validate [slug]",
		Short: "Pre-publish validation",
		Long:  `Generates the Xcode project to verify the app configuration is valid before publishing.`,
		Example: fmt.Sprintf(`  %s publish validate <slug>
  %s publish validate --app-id <uuid>`, cliName, cliName),
		Args: cobra.MaximumNArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			return runValidate(c, args)
		},
	}
}

func runValidate(c *cobra.Command, args []string) error {
	appID, err := config.ResolveAppIdentifier(c, args...)
	if err != nil {
		return err
	}

	client, err := config.AuthenticatedClient(c)
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
