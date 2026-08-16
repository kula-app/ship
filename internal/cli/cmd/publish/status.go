package cmd_publish

import (
	"fmt"

	"github.com/getsentry/sentry-go"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"

	"github.com/kula-app/ship/internal/cli/config"
	"github.com/kula-app/ship/internal/cli/helpers"
)

// PublishStatusCommandDeps declares the dependencies required by the publish status command.
type PublishStatusCommandDeps = PublishCommandsDeps

// newStatusCmd creates the "publish status" command.
func newStatusCmd(cliName string, deps PublishStatusCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show publish job status",
		Long:  `Show the status of the app's current publish job and its individual tasks.`,
		Example: fmt.Sprintf(`  # Check the publish status
  %[1]s publish status --app-id <uuid>

  # JSON output for scripting
  %[1]s publish status --app-id <uuid> --log-format json`, cliName),
	}

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runPublishStatus(cmd, deps, cliName)
	}

	return cmd
}

func runPublishStatus(cmd *cobra.Command, deps PublishStatusCommandDeps, cliName string) error {
	// Start root Sentry transaction for CLI command
	transaction := helpers.StartCommandTransaction(cmd, fmt.Sprintf("%s publish status", cliName))
	transaction.SetData("command", "publish.status")
	transaction.SetData("cli_name", cliName)
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

	// Get API credentials
	logger.InfoContext(ctx, "retrieving API credentials")
	svc, credentials, err := helpers.ResolveShipService(cmd, deps)
	if err != nil {
		return helpers.FailUnauthenticated(transaction, err)
	}
	transaction.SetData("api_url", credentials.APIURL)

	status, body, err := svc.GetPublishStatus(ctx, appID)
	if err != nil {
		return helpers.FailInternal(transaction, err)
	}
	transaction.SetData("publish_status", status.Status)

	// Output final result
	if outputJSON {
		transaction.Status = sentry.SpanStatusOK
		return helpers.OutputJSON(cmd, body)
	}

	// Text output
	helpers.PrintMessage(cmd, "Status: %s", status.Status)

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
		return helpers.FailInternal(transaction, err)
	}

	transaction.Status = sentry.SpanStatusOK
	return nil
}
