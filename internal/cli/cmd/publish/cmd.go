package cmd_publish

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/kula-app/ship/internal/cli/config"
)

type publishJobRequest struct {
	Platforms []string `json:"platforms,omitempty"`
}

type publishJobResponse struct {
	JobID string `json:"job_id"`
	IsNew bool   `json:"is_new"`
}

// NewPublishCmd creates and returns the publish command with all subcommands.
func NewPublishCmd(cliName string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "publish [slug]",
		Short: "Publish an app",
		Long:  `Trigger a full publish workflow for a Shipable app. Use subcommands for partial publishes.`,
		Example: fmt.Sprintf(`  %s publish <slug>
  %s publish --app-id <uuid>
  %s publish --app-slug <slug> --platform ios
  %s publish metadata <slug>`, cliName, cliName, cliName, cliName),
		Args: cobra.MaximumNArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			return RunFullPublish(c, args)
		},
	}

	cmd.PersistentFlags().String("app-id", "", "App ID (UUID); env: SHIP_APP_ID")
	cmd.PersistentFlags().String("app-slug", "", "App slug, alternative to --app-id; env: SHIP_APP_SLUG")
	cmd.PersistentFlags().StringSlice("platform", nil, "Target platforms (ios, android); omit for all")
	cmd.MarkFlagsMutuallyExclusive("app-id", "app-slug")

	cmd.AddCommand(newMetadataCmd(cliName))
	cmd.AddCommand(newScreenshotsCmd(cliName))
	cmd.AddCommand(newAppCmd(cliName))
	cmd.AddCommand(newStatusCmd(cliName))
	cmd.AddCommand(newValidateCmd(cliName))

	return cmd
}

// RunFullPublish triggers a full publish workflow for the resolved app.
func RunFullPublish(c *cobra.Command, args []string) error {
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
		fmt.Sprintf("/api/app/%s/publish", appID),
		publishJobRequest{Platforms: platforms},
	)
	if err != nil {
		return fmt.Errorf("failed to trigger publish: %w", err)
	}

	return printJobResponse(c, body)
}

func newMetadataCmd(cliName string) *cobra.Command {
	return &cobra.Command{
		Use:   "metadata [slug]",
		Short: "Publish metadata only",
		Example: fmt.Sprintf(`  %s publish metadata <slug>
  %s publish metadata --app-id <uuid>`, cliName, cliName),
		Args: cobra.MaximumNArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			return runPartialPublish(c, "metadata", args)
		},
	}
}

func newScreenshotsCmd(cliName string) *cobra.Command {
	return &cobra.Command{
		Use:   "screenshots [slug]",
		Short: "Publish screenshots only",
		Example: fmt.Sprintf(`  %s publish screenshots <slug>
  %s publish screenshots --app-id <uuid>`, cliName, cliName),
		Args: cobra.MaximumNArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			return runPartialPublish(c, "screenshots", args)
		},
	}
}

func newAppCmd(cliName string) *cobra.Command {
	return &cobra.Command{
		Use:   "app [slug]",
		Short: "Publish app binary only",
		Example: fmt.Sprintf(`  %s publish app <slug>
  %s publish app --app-id <uuid>`, cliName, cliName),
		Args: cobra.MaximumNArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			return runPartialPublish(c, "app", args)
		},
	}
}

func runPartialPublish(c *cobra.Command, variant string, args []string) error {
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
		fmt.Sprintf("/api/app/%s/publish/%s", appID, variant),
		publishJobRequest{Platforms: platforms},
	)
	if err != nil {
		return fmt.Errorf("failed to trigger %s publish: %w", variant, err)
	}

	return printJobResponse(c, body)
}

func printJobResponse(c *cobra.Command, body []byte) error {
	logFormat, _ := c.Flags().GetString("log-format")
	if logFormat == "json" {
		fmt.Fprintln(os.Stdout, string(body))
		return nil
	}

	var resp publishJobResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if resp.IsNew {
		fmt.Fprintf(os.Stderr, "Publish job created: %s\n", resp.JobID)
	} else {
		fmt.Fprintf(os.Stderr, "Publish job already in progress: %s\n", resp.JobID)
	}

	fmt.Fprintln(os.Stdout, resp.JobID)
	return nil
}
