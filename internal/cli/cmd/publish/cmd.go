package cmd_publish

import (
	"fmt"

	"github.com/getsentry/sentry-go"
	"github.com/spf13/cobra"

	"github.com/kula-app/ship/internal/cli/bootstrap"
	"github.com/kula-app/ship/internal/cli/config"
	"github.com/kula-app/ship/internal/cli/helpers"
	"github.com/kula-app/ship/internal/cli/service"
)

// PublishCommandsDeps declares the dependencies shared by the publish subcommands.
type PublishCommandsDeps interface {
	bootstrap.LoggerFactory
	service.ShipServiceFactory
}

// NewPublishCmd creates and returns the publish command with all subcommands.
func NewPublishCmd(cliName string, deps PublishCommandsDeps) *cobra.Command {
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
	}

	cmd.PersistentFlags().String("app-id", "", "App ID (UUID); env: SHIP_APP_ID")
	cmd.PersistentFlags().String("app-slug", "", "App slug, alternative to --app-id; env: SHIP_APP_SLUG")
	cmd.PersistentFlags().StringSlice("platform", nil, "Target platforms (ios, android); omit for all")
	cmd.MarkFlagsMutuallyExclusive("app-id", "app-slug")

	cmd.AddCommand(newMetadataCmd(cliName, deps))
	cmd.AddCommand(newScreenshotsCmd(cliName, deps))
	cmd.AddCommand(newAppCmd(cliName, deps))
	cmd.AddCommand(newStatusCmd(cliName, deps))
	cmd.AddCommand(newValidateCmd(cliName, deps))

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runPublish(cmd, deps, cliName)
	}

	return cmd
}

func runPublish(cmd *cobra.Command, deps PublishCommandsDeps, cliName string) error {
	return runPublishJob(cmd, deps, fmt.Sprintf("%s publish", cliName), "publish",
		func(svc *service.ShipService, appID string, platforms []string) (*service.PublishJobResult, []byte, error) {
			return svc.TriggerPublish(cmd.Context(), appID, platforms)
		})
}

// runPublishJob carries the flow shared by every publish variant: resolve the
// app and credentials, trigger the job through the service, then report it.
func runPublishJob(
	cmd *cobra.Command,
	deps PublishCommandsDeps,
	transactionName string,
	variant string,
	trigger func(svc *service.ShipService, appID string, platforms []string) (*service.PublishJobResult, []byte, error),
) error {
	// Start root Sentry transaction for CLI command
	transaction := helpers.StartCommandTransaction(cmd, transactionName)
	transaction.SetData("command", "publish."+variant)
	transaction.SetData("variant", variant)
	defer transaction.Finish()

	ctx := cmd.Context()
	logger := deps.GetLogger()

	// Get log format from global flag
	outputJSON, err := helpers.ResolveLogFormat(cmd)
	if err != nil {
		return helpers.FailInvalidArgument(transaction, err)
	}
	transaction.SetData("output_json", outputJSON)

	appID, err := config.ResolveAppIdentifier(cmd)
	if err != nil {
		return helpers.FailInvalidArgument(transaction, err)
	}
	transaction.SetData("app_id", appID)

	platforms, err := cmd.Flags().GetStringSlice("platform")
	if err != nil {
		return helpers.FailInvalidArgument(transaction, fmt.Errorf("failed to get platform flag: %w", err))
	}
	transaction.SetData("platforms", platforms)

	// Get API credentials
	logger.InfoContext(ctx, "retrieving API credentials")
	svc, credentials, err := helpers.ResolveShipService(cmd, deps)
	if err != nil {
		return helpers.FailUnauthenticated(transaction, err)
	}
	transaction.SetData("api_url", credentials.APIURL)

	result, body, err := trigger(svc, appID, platforms)
	if err != nil {
		return helpers.FailInternal(transaction, err)
	}
	transaction.SetData("job_id", result.JobID)
	transaction.SetData("is_new", result.IsNew)

	// Output final result
	if outputJSON {
		transaction.Status = sentry.SpanStatusOK
		return helpers.OutputJSON(cmd, body)
	}

	// Text output
	if result.IsNew {
		helpers.PrintMessage(cmd, "Publish job created: %s", result.JobID)
	} else {
		helpers.PrintMessage(cmd, "Publish job already in progress: %s", result.JobID)
	}
	fmt.Fprintln(cmd.OutOrStdout(), result.JobID)

	transaction.Status = sentry.SpanStatusOK
	return nil
}
