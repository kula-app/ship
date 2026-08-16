package cmd

import (
	"fmt"

	"github.com/getsentry/sentry-go"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"

	"github.com/kula-app/ship/internal/cli/config"
	"github.com/kula-app/ship/internal/cli/helpers"
	"github.com/kula-app/ship/internal/cli/service"
)

// newPublishCmd creates and returns the publish command with all subcommands.
func newPublishCmd(cliName string, deps ShipCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "publish",
		Short: "Publish an app",
		Long:  `Trigger a full publish workflow for a Shipable app. Use subcommands for partial publishes.`,
		Example: fmt.Sprintf(`  # Publish everything
  %[1]s publish --app-id <uuid>

  # Publish a single platform
  %[1]s publish --app-slug <slug> --platform ios

  # Publish only the metadata
  %[1]s publish metadata --app-id <uuid>`, cliName),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPublish(cmd, deps, cliName)
		},
	}

	cmd.PersistentFlags().String("app-id", "", "App ID (UUID); env: SHIP_APP_ID")
	cmd.PersistentFlags().String("app-slug", "", "App slug, alternative to --app-id; env: SHIP_APP_SLUG")
	cmd.PersistentFlags().StringSlice("platform", nil, "Target platforms (ios, android); omit for all")
	cmd.MarkFlagsMutuallyExclusive("app-id", "app-slug")

	cmd.AddCommand(newPublishMetadataCmd(cliName, deps))
	cmd.AddCommand(newPublishScreenshotsCmd(cliName, deps))
	cmd.AddCommand(newPublishAppCmd(cliName, deps))
	cmd.AddCommand(newPublishStatusCmd(cliName, deps))
	cmd.AddCommand(newPublishValidateCmd(cliName, deps))

	return cmd
}

func newPublishMetadataCmd(cliName string, deps ShipCommandDeps) *cobra.Command {
	return &cobra.Command{
		Use:     "metadata",
		Short:   "Publish metadata only",
		Example: fmt.Sprintf(`  %[1]s publish metadata --app-id <uuid>`, cliName),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPartialPublish(cmd, deps, cliName, "metadata")
		},
	}
}

func newPublishScreenshotsCmd(cliName string, deps ShipCommandDeps) *cobra.Command {
	return &cobra.Command{
		Use:     "screenshots",
		Short:   "Publish screenshots only",
		Example: fmt.Sprintf(`  %[1]s publish screenshots --app-id <uuid>`, cliName),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPartialPublish(cmd, deps, cliName, "screenshots")
		},
	}
}

func newPublishAppCmd(cliName string, deps ShipCommandDeps) *cobra.Command {
	return &cobra.Command{
		Use:     "app",
		Short:   "Publish app binary only",
		Example: fmt.Sprintf(`  %[1]s publish app --app-id <uuid>`, cliName),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPartialPublish(cmd, deps, cliName, "app")
		},
	}
}

func newPublishValidateCmd(cliName string, deps ShipCommandDeps) *cobra.Command {
	return &cobra.Command{
		Use:     "validate",
		Short:   "Pre-publish validation",
		Long:    `Generates the Xcode project to verify the app configuration is valid before publishing.`,
		Example: fmt.Sprintf(`  %[1]s publish validate --app-id <uuid>`, cliName),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPublishValidate(cmd, deps, cliName)
		},
	}
}

func newPublishStatusCmd(cliName string, deps ShipCommandDeps) *cobra.Command {
	return &cobra.Command{
		Use:     "status",
		Short:   "Show publish job status",
		Example: fmt.Sprintf(`  %[1]s publish status --app-id <uuid>`, cliName),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPublishStatus(cmd, deps, cliName)
		},
	}
}

func runPublish(cmd *cobra.Command, deps ShipCommandDeps, cliName string) error {
	return runPublishJob(cmd, deps, fmt.Sprintf("%s publish", cliName), "publish",
		func(svc *service.ShipService, appID string, platforms []string) (*service.PublishJobResult, []byte, error) {
			return svc.TriggerPublish(cmd.Context(), appID, platforms)
		})
}

func runPartialPublish(cmd *cobra.Command, deps ShipCommandDeps, cliName, variant string) error {
	return runPublishJob(cmd, deps, fmt.Sprintf("%s publish %s", cliName, variant), variant,
		func(svc *service.ShipService, appID string, platforms []string) (*service.PublishJobResult, []byte, error) {
			return svc.TriggerPartialPublish(cmd.Context(), appID, variant, platforms)
		})
}

func runPublishValidate(cmd *cobra.Command, deps ShipCommandDeps, cliName string) error {
	return runPublishJob(cmd, deps, fmt.Sprintf("%s publish validate", cliName), "validate",
		func(svc *service.ShipService, appID string, platforms []string) (*service.PublishJobResult, []byte, error) {
			return svc.TriggerValidation(cmd.Context(), appID, platforms)
		})
}

// runPublishJob carries the flow shared by every publish variant: resolve the
// app and credentials, trigger the job through the service, then report it.
func runPublishJob(
	cmd *cobra.Command,
	deps ShipCommandDeps,
	transactionName string,
	variant string,
	trigger func(svc *service.ShipService, appID string, platforms []string) (*service.PublishJobResult, []byte, error),
) error {
	// Start root Sentry transaction for CLI command
	transaction := helpers.StartCommandTransaction(cmd, transactionName)
	transaction.SetData("command", transactionName)
	transaction.SetData("variant", variant)
	defer transaction.Finish()

	ctx := cmd.Context()
	logger := deps.GetLogger()

	// Get log format from global flag
	outputJson, err := resolveLogFormat(cmd)
	if err != nil {
		return failInvalidArgument(transaction, err)
	}
	transaction.SetData("log_format", outputJson)

	appID, err := config.ResolveAppIdentifier(cmd)
	if err != nil {
		return failInvalidArgument(transaction, err)
	}
	transaction.SetData("app_id", appID)

	platforms, err := cmd.Flags().GetStringSlice("platform")
	if err != nil {
		return failInvalidArgument(transaction, fmt.Errorf("failed to get platforms: %w", err))
	}
	transaction.SetData("platforms", platforms)

	// Get API credentials
	logger.InfoContext(ctx, "retrieving API credentials")
	svc, credentials, err := resolveShipService(cmd, deps)
	if err != nil {
		return failUnauthenticated(transaction, err)
	}
	transaction.SetData("api_url", credentials.APIURL)

	result, body, err := trigger(svc, appID, platforms)
	if err != nil {
		return failInternal(transaction, err)
	}
	transaction.SetData("job_id", result.JobID)
	transaction.SetData("is_new", result.IsNew)

	// Output final result
	if outputJson {
		transaction.Status = sentry.SpanStatusOK
		return outputJSON(cmd, body)
	}

	// Text output
	if result.IsNew {
		printMessage(cmd, "Publish job created: %s", result.JobID)
	} else {
		printMessage(cmd, "Publish job already in progress: %s", result.JobID)
	}
	fmt.Fprintln(cmd.OutOrStdout(), result.JobID)

	transaction.Status = sentry.SpanStatusOK
	return nil
}

func runPublishStatus(cmd *cobra.Command, deps ShipCommandDeps, cliName string) error {
	// Start root Sentry transaction for CLI command
	transaction := helpers.StartCommandTransaction(cmd, fmt.Sprintf("%s publish status", cliName))
	transaction.SetData("command", "publish status")
	transaction.SetData("cli_name", cliName)
	defer transaction.Finish()

	ctx := cmd.Context()
	logger := deps.GetLogger()

	// Get log format from global flag
	outputJson, err := resolveLogFormat(cmd)
	if err != nil {
		return failInvalidArgument(transaction, err)
	}
	transaction.SetData("log_format", outputJson)

	appID, err := config.ResolveAppIdentifier(cmd)
	if err != nil {
		return failInvalidArgument(transaction, err)
	}
	transaction.SetData("app_id", appID)

	// Get API credentials
	logger.InfoContext(ctx, "retrieving API credentials")
	svc, credentials, err := resolveShipService(cmd, deps)
	if err != nil {
		return failUnauthenticated(transaction, err)
	}
	transaction.SetData("api_url", credentials.APIURL)

	status, body, err := svc.GetPublishStatus(ctx, appID)
	if err != nil {
		return failInternal(transaction, err)
	}
	transaction.SetData("publish_status", status.Status)

	// Output final result
	if outputJson {
		transaction.Status = sentry.SpanStatusOK
		return outputJSON(cmd, body)
	}

	// Text output
	printMessage(cmd, "Status: %s", status.Status)

	if len(status.Tasks) == 0 {
		transaction.Status = sentry.SpanStatusOK
		return nil
	}

	table := tablewriter.NewTable(cmd.OutOrStdout())
	table.Header("Task", "Status")

	for name, task := range status.Tasks {
		table.Append(name, task.Status)
	}

	if err := table.Render(); err != nil {
		return failInternal(transaction, err)
	}

	transaction.Status = sentry.SpanStatusOK
	return nil
}
