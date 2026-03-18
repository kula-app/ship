package cmd_auth

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/kula-app/ship/internal/cli/auth"
	"github.com/kula-app/ship/internal/cli/config"
	"github.com/kula-app/ship/internal/cli/db"
)

const loginTimeout = 5 * time.Minute

// newLoginCmd creates the "auth login" command.
func newLoginCmd(cliName string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Authenticate with Shipable",
		Long: `Authenticate with the Shipable API using your browser.

This opens your default browser to complete the OAuth login flow.
After successful authentication, credentials are stored locally
in ~/.ship/cli.db for future use.`,
		Example: fmt.Sprintf(`  %s auth login`, cliName),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLogin(cmd, args, cliName)
		},
	}

	return cmd
}

func runLogin(cmd *cobra.Command, _ []string, _ string) error {
	logger := slog.Default()

	// Check if already authenticated
	if db.IsAuthenticated() {
		fmt.Fprintln(os.Stderr, "You are already authenticated. Proceeding will replace existing credentials.")
	}

	// Resolve API URL from config
	apiURL := config.ResolveAPIURL()

	// Discover OAuth endpoints
	logger.Info("Discovering authentication endpoints...")
	endpoints, err := auth.DiscoverAuthEndpoints(apiURL)
	if err != nil {
		return fmt.Errorf("failed to discover authentication endpoints: %w", err)
	}

	// Generate PKCE verifier and challenge
	codeVerifier, err := auth.GenerateCodeVerifier()
	if err != nil {
		return fmt.Errorf("failed to generate PKCE verifier: %w", err)
	}
	codeChallenge := auth.GenerateCodeChallenge(codeVerifier)

	// Start local callback server
	ctx, cancel := context.WithTimeout(cmd.Context(), loginTimeout)
	defer cancel()

	resultChan, err := auth.StartCallbackServer(ctx)
	if err != nil {
		return fmt.Errorf("failed to start callback server: %w", err)
	}

	// Build authorization URL
	authURL, err := buildAuthURL(endpoints.AuthorizationEndpoint, codeChallenge)
	if err != nil {
		return fmt.Errorf("failed to build authorization URL: %w", err)
	}

	// Open browser
	fmt.Fprintln(os.Stderr, "Opening browser for authentication...")
	if err := auth.OpenBrowser(authURL); err != nil {
		logger.Warn("Failed to open browser automatically", "error", err)
	}

	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "If the browser did not open, visit this URL manually:")
	fmt.Fprintln(os.Stderr, authURL)
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "Waiting for authentication...")

	// Wait for callback
	select {
	case result := <-resultChan:
		if result.Error != "" {
			return fmt.Errorf("authentication failed: %s", result.Error)
		}

		// Exchange code for tokens
		logger.Info("Exchanging authorization code for tokens...")
		tokenResp, err := auth.ExchangeCode(endpoints.TokenEndpoint, result.Code, codeVerifier, auth.RedirectURI)
		if err != nil {
			return fmt.Errorf("failed to exchange authorization code: %w", err)
		}

		// Store tokens
		if err := db.SetAuthToken(tokenResp.AccessToken, tokenResp.RefreshToken, tokenResp.ExpiresIn); err != nil {
			return fmt.Errorf("failed to store credentials: %w", err)
		}

		fmt.Fprintln(os.Stderr, "Authentication successful!")
		return nil

	case <-ctx.Done():
		return fmt.Errorf("authentication timed out after %s", loginTimeout)
	}
}

// buildAuthURL constructs the full authorization URL with PKCE parameters.
func buildAuthURL(authEndpoint, codeChallenge string) (string, error) {
	u, err := url.Parse(authEndpoint)
	if err != nil {
		return "", err
	}

	q := u.Query()
	q.Set("client_id", auth.ClientID)
	q.Set("response_type", "code")
	q.Set("redirect_uri", auth.RedirectURI)
	q.Set("code_challenge", codeChallenge)
	q.Set("code_challenge_method", "S256")
	u.RawQuery = q.Encode()

	return u.String(), nil
}
