// Package config provides shared configuration helpers for the CLI.
package config

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/kula-app/ship/internal/cli/api"
	"github.com/kula-app/ship/internal/cli/auth"
	"github.com/kula-app/ship/internal/cli/db"
)

const DefaultAPIURL = "https://api.shipable.dev"

// ResolveAPIURL determines the API URL from the global flag, environment, or
// database settings, in that order of priority.
func ResolveAPIURL(cmd *cobra.Command) string {
	if cmd != nil {
		apiURL, err := cmd.Root().PersistentFlags().GetString("api-url")
		if err == nil && apiURL != "" {
			return apiURL
		}
	}

	if envURL := os.Getenv("SHIP_API_URL"); envURL != "" {
		return envURL
	}

	if apiURL, err := db.GetSetting("api_url"); err == nil && apiURL != "" {
		return apiURL
	}

	return DefaultAPIURL
}

// apiKeyFlag returns the value of the explicit --api-key flag, or "" if unset.
func apiKeyFlag(cmd *cobra.Command) string {
	if cmd == nil {
		return ""
	}
	apiKey, err := cmd.Root().PersistentFlags().GetString("api-key")
	if err != nil {
		return ""
	}
	return apiKey
}

// ResolveAPIKey determines the API key from the global flag or environment.
// The explicit --api-key flag takes priority over the SHIP_API_KEY env var.
func ResolveAPIKey(cmd *cobra.Command) string {
	if apiKey := apiKeyFlag(cmd); apiKey != "" {
		return apiKey
	}

	return os.Getenv("SHIP_API_KEY")
}

// AuthenticatedClient returns an API client using stored credentials.
// Returns an error if the user is not authenticated.
func AuthenticatedClient(cmd *cobra.Command) (*api.Client, error) {
	apiURL := ResolveAPIURL(cmd)
	rootName := "ship"
	if cmd != nil {
		rootName = cmd.Root().Name()
	}

	// An explicit --api-key flag always wins, even over a valid session token.
	// The SHIP_API_KEY env var only applies as a fallback when no session exists.
	if apiKey := apiKeyFlag(cmd); apiKey != "" {
		return api.NewAPIKeyClient(apiURL, apiKey), nil
	}

	credentials, err := db.GetAuthCredentials()
	if err != nil {
		if apiKey := ResolveAPIKey(cmd); apiKey != "" {
			return api.NewAPIKeyClient(apiURL, apiKey), nil
		}
		return nil, fmt.Errorf("failed to read credentials: %w", err)
	}
	if credentials == nil || credentials.AccessToken == "" {
		if apiKey := ResolveAPIKey(cmd); apiKey != "" {
			return api.NewAPIKeyClient(apiURL, apiKey), nil
		}
		return nil, fmt.Errorf("not authenticated — run '%s auth login' first or set SHIP_API_KEY", rootName)
	}
	if !credentials.IsExpired(time.Now()) {
		return api.NewClient(apiURL, credentials.AccessToken), nil
	}

	if credentials.RefreshToken == "" {
		if apiKey := ResolveAPIKey(cmd); apiKey != "" {
			return api.NewAPIKeyClient(apiURL, apiKey), nil
		}
		return nil, fmt.Errorf("credentials expired — run '%s auth login' first or set SHIP_API_KEY", rootName)
	}

	endpoints, err := auth.DiscoverAuthEndpoints(apiURL)
	if err != nil {
		return nil, fmt.Errorf("credentials expired and refresh endpoint discovery failed: %w", err)
	}

	tokenResp, err := auth.RefreshToken(endpoints.TokenEndpoint, credentials.RefreshToken)
	if err != nil {
		return nil, fmt.Errorf("credentials expired and refresh failed — run '%s auth login' first: %w", rootName, err)
	}

	refreshToken := tokenResp.RefreshToken
	if refreshToken == "" {
		refreshToken = credentials.RefreshToken
	}
	if err := db.SetAuthToken(tokenResp.AccessToken, refreshToken, tokenResp.ExpiresIn); err != nil {
		return nil, fmt.Errorf("failed to store refreshed credentials: %w", err)
	}

	return api.NewClient(apiURL, tokenResp.AccessToken), nil
}
