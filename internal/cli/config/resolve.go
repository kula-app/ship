// Package config provides shared configuration helpers for the CLI.
package config

import (
	"fmt"
	"os"

	"github.com/kula-app/ship/internal/cli/api"
	"github.com/kula-app/ship/internal/cli/db"
)

const DefaultAPIURL = "https://api.shipable.dev"

// ResolveAPIURL determines the API URL from environment or database settings.
func ResolveAPIURL() string {
	if envURL := os.Getenv("SHIPABLE_API_URL"); envURL != "" {
		return envURL
	}

	if apiURL, err := db.GetSetting("api_url"); err == nil && apiURL != "" {
		return apiURL
	}

	return DefaultAPIURL
}

// AuthenticatedClient returns an API client using stored credentials.
// Returns an error if the user is not authenticated.
func AuthenticatedClient(rootName string) (*api.Client, error) {
	token, err := db.GetAuthToken()
	if err != nil {
		return nil, fmt.Errorf("failed to read credentials: %w", err)
	}
	if token == "" {
		return nil, fmt.Errorf("not authenticated — run '%s auth login' first", rootName)
	}

	return api.NewClient(ResolveAPIURL(), token), nil
}
