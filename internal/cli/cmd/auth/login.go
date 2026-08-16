package cmd_auth

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/spf13/cobra"

	"github.com/kula-app/ship/internal/cli/auth"
	"github.com/kula-app/ship/internal/cli/bootstrap"
	"github.com/kula-app/ship/internal/cli/config"
	"github.com/kula-app/ship/internal/cli/db"
	"github.com/kula-app/ship/internal/cli/helpers"
)

// AuthLoginCommandDeps declares the dependencies required by the auth login command.
type AuthLoginCommandDeps interface {
	bootstrap.LoggerFactory
}

const loginTimeout = 5 * time.Minute

// newLoginCmd creates the "auth login" command.
func newLoginCmd(cliName string, deps AuthLoginCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Authenticate with Shipable",
		Long: `Authenticate with the Shipable API using your browser.

This opens your default browser to complete the OAuth login flow.
After successful authentication, credentials are stored locally
in ~/.ship/cli.db for future use.`,
		Example: fmt.Sprintf(`  %[1]s auth login`, cliName),
	}

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runAuthLogin(cmd, deps, cliName)
	}

	return cmd
}

func runAuthLogin(cmd *cobra.Command, deps AuthLoginCommandDeps, cliName string) error {
	// Start root Sentry transaction for CLI command
	transaction := helpers.StartCommandTransaction(cmd, fmt.Sprintf("%s auth login", cliName))
	transaction.SetData("command", "auth.login")
	transaction.SetData("cli_name", cliName)
	defer transaction.Finish()

	logger := deps.GetLogger()

	// Check if already authenticated
	if db.IsAuthenticated() {
		helpers.PrintMessage(cmd, "You are already authenticated. Proceeding will replace existing credentials.")
	}

	// Resolve API URL from config
	apiURL := config.ResolveAPIURL(cmd)
	transaction.SetData("api_url", apiURL)

	// The transaction context carries the trace, so it must be the parent of the
	// login timeout rather than the pre-transaction command context.
	ctx, cancel := context.WithTimeout(cmd.Context(), loginTimeout)
	defer cancel()

	// Discover OAuth endpoints
	logger.InfoContext(ctx, "discovering authentication endpoints")
	endpoints, err := auth.DiscoverAuthEndpoints(apiURL)
	if err != nil {
		return helpers.FailInternal(transaction, fmt.Errorf("failed to discover authentication endpoints: %w", err))
	}

	// Generate PKCE verifier and challenge
	codeVerifier, err := auth.GenerateCodeVerifier()
	if err != nil {
		return helpers.FailInternal(transaction, fmt.Errorf("failed to generate PKCE verifier: %w", err))
	}
	codeChallenge := auth.GenerateCodeChallenge(codeVerifier)

	state, err := auth.GenerateState()
	if err != nil {
		return helpers.FailInternal(transaction, fmt.Errorf("failed to generate OAuth state: %w", err))
	}

	// Start local callback server
	resultChan, err := auth.StartCallbackServer(ctx)
	if err != nil {
		return helpers.FailInternal(transaction, fmt.Errorf("failed to start callback server: %w", err))
	}

	// Build authorization URL
	authURL, err := buildAuthURL(endpoints.AuthorizationEndpoint, codeChallenge, state)
	if err != nil {
		return helpers.FailInternal(transaction, fmt.Errorf("failed to build authorization URL: %w", err))
	}

	// Open browser
	helpers.PrintMessage(cmd, "Opening browser for authentication...")
	if err := auth.OpenBrowser(authURL); err != nil {
		logger.WarnContext(ctx, "failed to open browser automatically", "error", err)
	}

	helpers.PrintMessage(cmd, "")
	helpers.PrintMessage(cmd, "If the browser did not open, visit this URL manually:")
	helpers.PrintMessage(cmd, "%s", authURL)
	helpers.PrintMessage(cmd, "")
	helpers.PrintMessage(cmd, "Waiting for authentication...")

	// Wait for callback
	select {
	case result := <-resultChan:
		if result.State != "" && result.State != state {
			return helpers.FailInvalidArgument(transaction, fmt.Errorf("authentication failed: invalid OAuth state"))
		}
		if result.Error != "" {
			return helpers.FailUnauthenticated(transaction, fmt.Errorf("authentication failed: %s", result.Error))
		}
		if result.State == "" {
			return helpers.FailInvalidArgument(transaction, fmt.Errorf("authentication failed: missing OAuth state"))
		}

		// Exchange code for tokens
		logger.InfoContext(ctx, "exchanging authorization code for tokens")
		tokenResp, err := auth.ExchangeCode(endpoints.TokenEndpoint, result.Code, codeVerifier, auth.RedirectURI)
		if err != nil {
			return helpers.FailInternal(transaction, fmt.Errorf("failed to exchange authorization code: %w", err))
		}

		// Store tokens
		if err := db.SetAuthToken(tokenResp.AccessToken, tokenResp.RefreshToken, tokenResp.ExpiresIn); err != nil {
			return helpers.FailInternal(transaction, fmt.Errorf("failed to store credentials: %w", err))
		}

		logger.InfoContext(ctx, "authentication successful")
		helpers.PrintMessage(cmd, "Authentication successful!")

		transaction.Status = sentry.SpanStatusOK
		return nil

	case <-ctx.Done():
		transaction.Status = sentry.SpanStatusDeadlineExceeded
		return fmt.Errorf("authentication timed out after %s", loginTimeout)
	}
}

// buildAuthURL constructs the full authorization URL with PKCE parameters.
func buildAuthURL(authEndpoint, codeChallenge, state string) (string, error) {
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
	q.Set("scope", auth.DefaultScope)
	q.Set("state", state)
	u.RawQuery = q.Encode()

	return u.String(), nil
}
