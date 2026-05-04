package cmd_publish

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
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
		Use:   "status [slug]",
		Short: "Show publish job status",
		Example: fmt.Sprintf(`  %s publish status --app-id <uuid>
  %s publish status gritch`, cliName, cliName),
		Args: cobra.MaximumNArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			return runStatus(c, args)
		},
	}
}

func runStatus(c *cobra.Command, args []string) error {
	appID, err := resolveAppID(c, args)
	if err != nil {
		return err
	}

	client, err := authenticatedClient(c)
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
