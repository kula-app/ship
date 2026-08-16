package cmd

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

// AuthCommandDeps declares the dependencies required by the auth commands.
// They manage local credentials rather than calling the API, so a logger is all
// they need.
type AuthCommandDeps interface {
	bootstrap.LoggerFactory
}

const loginTimeout = 5 * time.Minute

// newAuthCmd creates and returns the auth command with all subcommands.
func newAuthCmd(cliName string, deps AuthCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Manage authentication",
		Long:  `Commands for managing authentication credentials for the Shipable API.`,
	}

	cmd.AddCommand(newLoginCmd(cliName, deps))
	cmd.AddCommand(newLogoutCmd(cliName, deps))

	return cmd
}

// newLoginCmd creates the "auth login" command.
func newLoginCmd(cliName string, deps AuthCommandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "login",
		Short: "Authenticate with Shipable",
		Long: `Authenticate with the Shipable API using your browser.

This opens your default browser to complete the OAuth login flow.
After successful authentication, credentials are stored locally
in ~/.ship/cli.db for future use.`,
		Example: fmt.Sprintf(`  %[1]s auth login`, cliName),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLogin(cmd, deps, cliName)
		},
	}
}

// newLogoutCmd creates the "auth logout" command.
func newLogoutCmd(cliName string, deps AuthCommandDeps) *cobra.Command {
	return &cobra.Command{
		Use:     "logout",
		Short:   "Log out of Shipable",
		Long:    `Remove locally stored authentication credentials.`,
		Example: fmt.Sprintf(`  %[1]s auth logout`, cliName),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLogout(cmd, deps, cliName)
		},
	}
}

func runLogin(cmd *cobra.Command, deps AuthCommandDeps, cliName string) error {
	// Start root Sentry transaction for CLI command
	transaction := helpers.StartCommandTransaction(cmd, fmt.Sprintf("%s auth login", cliName))
	transaction.SetData("command", "auth login")
	transaction.SetData("cli_name", cliName)
	defer transaction.Finish()

	logger := deps.GetLogger()

	// Check if already authenticated
	if db.IsAuthenticated() {
		printMessage(cmd, "You are already authenticated. Proceeding will replace existing credentials.")
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
		return failInternal(transaction, fmt.Errorf("failed to discover authentication endpoints: %w", err))
	}

	// Generate PKCE verifier and challenge
	codeVerifier, err := auth.GenerateCodeVerifier()
	if err != nil {
		return failInternal(transaction, fmt.Errorf("failed to generate PKCE verifier: %w", err))
	}
	codeChallenge := auth.GenerateCodeChallenge(codeVerifier)

	state, err := auth.GenerateState()
	if err != nil {
		return failInternal(transaction, fmt.Errorf("failed to generate OAuth state: %w", err))
	}

	// Start local callback server
	resultChan, err := auth.StartCallbackServer(ctx)
	if err != nil {
		return failInternal(transaction, fmt.Errorf("failed to start callback server: %w", err))
	}

	// Build authorization URL
	authURL, err := buildAuthURL(endpoints.AuthorizationEndpoint, codeChallenge, state)
	if err != nil {
		return failInternal(transaction, fmt.Errorf("failed to build authorization URL: %w", err))
	}

	// Open browser
	printMessage(cmd, "Opening browser for authentication...")
	if err := auth.OpenBrowser(authURL); err != nil {
		logger.WarnContext(ctx, "failed to open browser automatically", "error", err)
	}

	printMessage(cmd, "")
	printMessage(cmd, "If the browser did not open, visit this URL manually:")
	printMessage(cmd, "%s", authURL)
	printMessage(cmd, "")
	printMessage(cmd, "Waiting for authentication...")

	// Wait for callback
	select {
	case result := <-resultChan:
		if result.State != "" && result.State != state {
			return failInvalidArgument(transaction, fmt.Errorf("authentication failed: invalid OAuth state"))
		}
		if result.Error != "" {
			return failUnauthenticated(transaction, fmt.Errorf("authentication failed: %s", result.Error))
		}
		if result.State == "" {
			return failInvalidArgument(transaction, fmt.Errorf("authentication failed: missing OAuth state"))
		}

		// Exchange code for tokens
		logger.InfoContext(ctx, "exchanging authorization code for tokens")
		tokenResp, err := auth.ExchangeCode(endpoints.TokenEndpoint, result.Code, codeVerifier, auth.RedirectURI)
		if err != nil {
			return failInternal(transaction, fmt.Errorf("failed to exchange authorization code: %w", err))
		}

		// Store tokens
		if err := db.SetAuthToken(tokenResp.AccessToken, tokenResp.RefreshToken, tokenResp.ExpiresIn); err != nil {
			return failInternal(transaction, fmt.Errorf("failed to store credentials: %w", err))
		}

		logger.InfoContext(ctx, "authentication successful")
		printMessage(cmd, "Authentication successful!")

		transaction.Status = sentry.SpanStatusOK
		return nil

	case <-ctx.Done():
		transaction.Status = sentry.SpanStatusDeadlineExceeded
		return fmt.Errorf("authentication timed out after %s", loginTimeout)
	}
}

func runLogout(cmd *cobra.Command, deps AuthCommandDeps, cliName string) error {
	// Start root Sentry transaction for CLI command
	transaction := helpers.StartCommandTransaction(cmd, fmt.Sprintf("%s auth logout", cliName))
	transaction.SetData("command", "auth logout")
	transaction.SetData("cli_name", cliName)
	defer transaction.Finish()

	logger := deps.GetLogger()

	if !db.IsAuthenticated() {
		printMessage(cmd, "You are not currently authenticated.")
		transaction.Status = sentry.SpanStatusOK
		return nil
	}

	if err := db.ClearAuth(); err != nil {
		return failInternal(transaction, fmt.Errorf("failed to clear credentials: %w", err))
	}

	logger.InfoContext(cmd.Context(), "credentials cleared")
	printMessage(cmd, "Successfully logged out.")

	transaction.Status = sentry.SpanStatusOK
	return nil
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
