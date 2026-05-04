package cmd_publish

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/kula-app/ship/internal/cli/api"
	"github.com/kula-app/ship/internal/cli/config"
)

type appLookup struct {
	AppID string  `json:"app_id"`
	Slug  *string `json:"slug"`
}

func authenticatedClient(c *cobra.Command) (*api.Client, error) {
	return config.AuthenticatedClient(c.Root().Name())
}

func resolveAppID(c *cobra.Command, args []string) (string, error) {
	appID, err := c.Flags().GetString("app-id")
	if err == nil && appID != "" {
		return appID, nil
	}

	if len(args) == 0 {
		return "", fmt.Errorf("either --app-id or <slug> is required")
	}

	client, err := authenticatedClient(c)
	if err != nil {
		return "", err
	}

	body, err := client.Get("/api/apps/")
	if err != nil {
		return "", fmt.Errorf("failed to fetch apps: %w", err)
	}

	var apps []appLookup
	if err := json.Unmarshal(body, &apps); err != nil {
		return "", fmt.Errorf("failed to parse apps response: %w", err)
	}

	return matchAppIDBySlug(apps, args[0])
}

func matchAppIDBySlug(apps []appLookup, slug string) (string, error) {
	matchCount := 0
	appID := ""

	for _, app := range apps {
		if app.Slug == nil {
			continue
		}

		if *app.Slug != slug {
			continue
		}

		matchCount++
		appID = app.AppID
	}

	if matchCount == 1 {
		return appID, nil
	}

	if matchCount == 0 {
		return "", fmt.Errorf("app slug %q not found", slug)
	}

	return "", fmt.Errorf("app slug %q matched multiple apps; use --app-id", slug)
}
