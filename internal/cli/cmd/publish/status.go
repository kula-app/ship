package cmd_publish

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"

	"github.com/kula-app/ship/internal/cli/config"
)

type taskStatus struct {
	Status string `json:"status"`
}

type publishStatusResponse struct {
	Status string                `json:"status"`
	Tasks  map[string]taskStatus `json:"tasks,omitempty"`
}

func newStatusCmd(cliName string) *cobra.Command {
	return &cobra.Command{
		Use:     "status",
		Short:   "Show publish job status",
		Example: fmt.Sprintf(`  %s publish status --app-id <uuid>`, cliName),
		RunE: func(c *cobra.Command, args []string) error {
			return runStatus(c)
		},
	}
}

func runStatus(c *cobra.Command) error {
	appID, _ := c.Flags().GetString("app-id")

	client, err := config.AuthenticatedClient(c.Root().Name())
	if err != nil {
		return err
	}

	body, err := client.Get(fmt.Sprintf("/api/app/%s/publish/status", appID))
	if err != nil {
		return fmt.Errorf("failed to fetch publish status: %w", err)
	}

	logFormat, _ := c.Flags().GetString("log-format")
	if logFormat == "json" {
		fmt.Fprintln(os.Stdout, string(body))
		return nil
	}

	var resp publishStatusResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	fmt.Fprintf(os.Stderr, "Status: %s\n", resp.Status)

	if len(resp.Tasks) == 0 {
		return nil
	}

	table := tablewriter.NewTable(os.Stdout)
	table.Header("Task", "Status")

	for name, t := range resp.Tasks {
		table.Append(name, t.Status)
	}

	return table.Render()
}
