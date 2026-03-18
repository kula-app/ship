package cmd_apps

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"

	"github.com/kula-app/ship/internal/cli/config"
)

type app struct {
	AppID   string  `json:"app_id"`
	Slug    *string `json:"slug"`
	AppName *string `json:"app_name"`
}

// newListCmd creates the "apps list" command.
func newListCmd(cliName string) *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Short:   "List all apps",
		Long:    `Fetch and display all apps from the Shipable API.`,
		Example: fmt.Sprintf(`  %s apps list`, cliName),
		RunE: func(c *cobra.Command, args []string) error {
			return runList(c)
		},
	}
}

func runList(c *cobra.Command) error {
	client, err := config.AuthenticatedClient(c.Root().Name())
	if err != nil {
		return err
	}

	body, err := client.Get("/api/apps/")
	if err != nil {
		return fmt.Errorf("failed to fetch apps: %w", err)
	}

	logFormat, _ := c.Flags().GetString("log-format")
	if logFormat == "json" {
		fmt.Fprintln(os.Stdout, string(body))
		return nil
	}

	var apps []app
	if err := json.Unmarshal(body, &apps); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if len(apps) == 0 {
		fmt.Fprintln(os.Stderr, "No apps found.")
		return nil
	}

	table := tablewriter.NewTable(os.Stdout)
	table.Header("ID", "Slug", "Name")

	for _, a := range apps {
		slug := ""
		if a.Slug != nil {
			slug = *a.Slug
		}
		name := ""
		if a.AppName != nil {
			name = *a.AppName
		}
		table.Append(a.AppID, slug, name)
	}

	return table.Render()
}
