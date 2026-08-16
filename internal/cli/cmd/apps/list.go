package cmd_apps

import (
	"fmt"

	"github.com/getsentry/sentry-go"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"

	"github.com/kula-app/ship/internal/cli/bootstrap"
	"github.com/kula-app/ship/internal/cli/helpers"
	"github.com/kula-app/ship/internal/cli/service"
)

// AppsListCommandDeps declares the dependencies required by the apps list command.
type AppsListCommandDeps interface {
	bootstrap.LoggerFactory
	service.ShipServiceFactory
}

// newListCmd creates the "apps list" command.
func newListCmd(cliName string, deps AppsListCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all apps",
		Long:  `Fetch and display all apps from the Shipable API.`,
		Example: fmt.Sprintf(`  # List all apps
  %[1]s apps list

  # JSON output for scripting
  %[1]s apps list --log-format json`, cliName),
	}

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runAppsList(cmd, deps, cliName)
	}

	return cmd
}

func runAppsList(cmd *cobra.Command, deps AppsListCommandDeps, cliName string) error {
	// Start root Sentry transaction for CLI command
	transaction := helpers.StartCommandTransaction(cmd, fmt.Sprintf("%s apps list", cliName))
	transaction.SetData("command", "apps.list")
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

	// Get API credentials
	logger.InfoContext(ctx, "retrieving API credentials")
	svc, credentials, err := helpers.ResolveShipService(cmd, deps)
	if err != nil {
		return helpers.FailUnauthenticated(transaction, err)
	}
	transaction.SetData("api_url", credentials.APIURL)

	apps, body, err := svc.ListApps(ctx)
	if err != nil {
		return helpers.FailInternal(transaction, err)
	}
	transaction.SetData("app_count", len(apps))

	// Output final result
	if outputJSON {
		transaction.Status = sentry.SpanStatusOK
		return helpers.OutputJSON(cmd, body)
	}

	if len(apps) == 0 {
		helpers.PrintMessage(cmd, "No apps found.")
		transaction.Status = sentry.SpanStatusOK
		return nil
	}

	// Text output
	table := tablewriter.NewTable(cmd.OutOrStdout())
	table.Header("ID", "Slug", "Name")

	for _, app := range apps {
		slug := ""
		if app.Slug != nil {
			slug = *app.Slug
		}
		name := ""
		if app.AppName != nil {
			name = *app.AppName
		}
		table.Append(app.AppID, slug, name)
	}

	if err := table.Render(); err != nil {
		return helpers.FailInternal(transaction, err)
	}

	transaction.Status = sentry.SpanStatusOK
	return nil
}
