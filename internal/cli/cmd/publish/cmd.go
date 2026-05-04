package cmd_publish

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
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
		Example: fmt.Sprintf(`  %s publish --app-id <uuid>
  %s publish gritch
  %s publish --app-id <uuid> --platform ios
  %s publish metadata gritch`, cliName, cliName, cliName, cliName),
		Args: cobra.MaximumNArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			return runPublish(c, args)
		},
	}

	cmd.PersistentFlags().String("app-id", "", "App ID (required unless slug argument is provided)")
	cmd.PersistentFlags().StringSlice("platform", nil, "Target platforms (ios, android); omit for all")

	cmd.AddCommand(newMetadataCmd(cliName))
	cmd.AddCommand(newScreenshotsCmd(cliName))
	cmd.AddCommand(newAppCmd(cliName))
	cmd.AddCommand(newStatusCmd(cliName))
	cmd.AddCommand(newValidateCmd(cliName))

	return cmd
}

func RunRootPublish(c *cobra.Command, args []string) error {
	return runPublish(c, args)
}

func runPublish(c *cobra.Command, args []string) error {
	appID, err := resolveAppID(c, args)
	if err != nil {
		return err
	}

	client, err := authenticatedClient(c)
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
		Example: fmt.Sprintf(`  %s publish metadata --app-id <uuid>
  %s publish metadata gritch`, cliName, cliName),
		Args: cobra.MaximumNArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			return runPartialPublish(c, args, "metadata")
		},
	}
}

func newScreenshotsCmd(cliName string) *cobra.Command {
	return &cobra.Command{
		Use:   "screenshots [slug]",
		Short: "Publish screenshots only",
		Example: fmt.Sprintf(`  %s publish screenshots --app-id <uuid>
  %s publish screenshots gritch`, cliName, cliName),
		Args: cobra.MaximumNArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			return runPartialPublish(c, args, "screenshots")
		},
	}
}

func newAppCmd(cliName string) *cobra.Command {
	return &cobra.Command{
		Use:   "app [slug]",
		Short: "Publish app binary only",
		Example: fmt.Sprintf(`  %s publish app --app-id <uuid>
  %s publish app gritch`, cliName, cliName),
		Args: cobra.MaximumNArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			return runPartialPublish(c, args, "app")
		},
	}
}

func runPartialPublish(c *cobra.Command, args []string, variant string) error {
	appID, err := resolveAppID(c, args)
	if err != nil {
		return err
	}

	client, err := authenticatedClient(c)
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
