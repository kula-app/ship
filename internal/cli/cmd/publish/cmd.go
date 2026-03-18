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
		Use:   "publish",
		Short: "Publish an app",
		Long:  `Trigger a full publish workflow for a Shipable app. Use subcommands for partial publishes.`,
		Example: fmt.Sprintf(`  %s publish --app-id <uuid>
  %s publish --app-id <uuid> --platform ios
  %s publish metadata --app-id <uuid>`, cliName, cliName, cliName),
		RunE: func(c *cobra.Command, args []string) error {
			return runPublish(c)
		},
	}

	cmd.PersistentFlags().String("app-id", "", "App ID (required)")
	cmd.PersistentFlags().StringSlice("platform", nil, "Target platforms (ios, android); omit for all")
	_ = cmd.MarkPersistentFlagRequired("app-id")

	cmd.AddCommand(newMetadataCmd(cliName))
	cmd.AddCommand(newScreenshotsCmd(cliName))
	cmd.AddCommand(newAppCmd(cliName))
	cmd.AddCommand(newStatusCmd(cliName))
	cmd.AddCommand(newValidateCmd(cliName))

	return cmd
}

func runPublish(c *cobra.Command) error {
	appID, _ := c.Flags().GetString("app-id")

	client, err := config.AuthenticatedClient(c.Root().Name())
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
		Use:     "metadata",
		Short:   "Publish metadata only",
		Example: fmt.Sprintf(`  %s publish metadata --app-id <uuid>`, cliName),
		RunE: func(c *cobra.Command, args []string) error {
			return runPartialPublish(c, "metadata")
		},
	}
}

func newScreenshotsCmd(cliName string) *cobra.Command {
	return &cobra.Command{
		Use:     "screenshots",
		Short:   "Publish screenshots only",
		Example: fmt.Sprintf(`  %s publish screenshots --app-id <uuid>`, cliName),
		RunE: func(c *cobra.Command, args []string) error {
			return runPartialPublish(c, "screenshots")
		},
	}
}

func newAppCmd(cliName string) *cobra.Command {
	return &cobra.Command{
		Use:     "app",
		Short:   "Publish app binary only",
		Example: fmt.Sprintf(`  %s publish app --app-id <uuid>`, cliName),
		RunE: func(c *cobra.Command, args []string) error {
			return runPartialPublish(c, "app")
		},
	}
}

func runPartialPublish(c *cobra.Command, variant string) error {
	appID, _ := c.Flags().GetString("app-id")

	client, err := config.AuthenticatedClient(c.Root().Name())
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
