package cmd

import (
	"fmt"

	"github.com/getsentry/sentry-go"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"

	"github.com/kula-app/ship/internal/cli/bootstrap"
	"github.com/kula-app/ship/internal/cli/helpers"
	"github.com/kula-app/ship/internal/cli/service"
)

// ShipCommandDeps declares the dependencies required by commands that talk to
// the Shipable API.
type ShipCommandDeps interface {
	bootstrap.LoggerFactory
	service.ShipServiceFactory
}

// newAppsCmd creates and returns the apps command with all subcommands.
func newAppsCmd(cliName string, deps ShipCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "apps",
		Short: "Manage apps",
		Long:  `Commands for managing apps on Shipable.`,
	}

	cmd.AddCommand(newAppsListCmd(cliName, deps))

	return cmd
}

// newAppsListCmd creates the "apps list" command.
func newAppsListCmd(cliName string, deps ShipCommandDeps) *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Short:   "List all apps",
		Long:    `Fetch and display all apps from the Shipable API.`,
		Example: fmt.Sprintf(`  %[1]s apps list`, cliName),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAppsList(cmd, deps, cliName)
		},
	}
}

func runAppsList(cmd *cobra.Command, deps ShipCommandDeps, cliName string) error {
	// Start root Sentry transaction for CLI command
	transaction := helpers.StartCommandTransaction(cmd, fmt.Sprintf("%s apps list", cliName))
	transaction.SetData("command", "apps list")
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

	// Get API credentials
	logger.InfoContext(ctx, "retrieving API credentials")
	svc, credentials, err := resolveShipService(cmd, deps)
	if err != nil {
		return failUnauthenticated(transaction, err)
	}
	transaction.SetData("api_url", credentials.APIURL)

	apps, body, err := svc.ListApps(ctx)
	if err != nil {
		return failInternal(transaction, err)
	}
	transaction.SetData("app_count", len(apps))

	// Output final result
	if outputJson {
		transaction.Status = sentry.SpanStatusOK
		return outputJSON(cmd, body)
	}

	if len(apps) == 0 {
		printMessage(cmd, "No apps found.")
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
		return failInternal(transaction, err)
	}

	transaction.Status = sentry.SpanStatusOK
	return nil
}
