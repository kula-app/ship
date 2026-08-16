// Package cmd contains all CLI commands and their implementation.
// It uses the Cobra library for command-line interface construction.
//
// Command Structure:
//   - root: Base command with global flags
//   - api: Raw authenticated request against any API endpoint
//   - apps: List the apps available to the authenticated user
//   - auth: Log in and out, managing locally stored credentials
//   - publish: Trigger publish workflows and inspect their status
//
// Each command group lives in its own package under cmd/, with cmd.go holding
// the group constructor and one file per subcommand.
//
// Authentication:
// Credentials can be provided via:
//  1. --api-key flag (highest priority)
//  2. Stored session token from 'ship auth login' (refreshed when expired)
//  3. SHIP_API_KEY environment variable (lowest priority)
package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/kula-app/ship/internal/cli/bootstrap"
	cmd_api "github.com/kula-app/ship/internal/cli/cmd/api"
	cmd_apps "github.com/kula-app/ship/internal/cli/cmd/apps"
	cmd_auth "github.com/kula-app/ship/internal/cli/cmd/auth"
	cmd_publish "github.com/kula-app/ship/internal/cli/cmd/publish"
	"github.com/kula-app/ship/internal/cli/service"
)

// BuildMetadata holds build-time version information.
type BuildMetadata struct {
	Version string
	Commit  string
	Date    string
}

// RootCommandDeps declares the dependencies shared by root command groups.
type RootCommandDeps interface {
	bootstrap.LoggerFactory
	service.ShipServiceFactory
}

// NewRootCommand creates the fully-wired root command for the ship CLI using
// the default dependency container. It remains for compatibility with existing
// callers.
func NewRootCommand(cliName string, metadata BuildMetadata) *cobra.Command {
	return NewRootCommandWithDependencies(cliName, metadata, NewDependencyContainer())
}

// NewRootCommandWithDependencies creates the fully-wired root command using
// the supplied dependencies. All subcommands are constructed on demand via
// factory functions, with build metadata and CLI name passed as parameters.
func NewRootCommandWithDependencies(cliName string, metadata BuildMetadata, deps RootCommandDeps) *cobra.Command {
	rootCmd := &cobra.Command{
		Use:          cliName,
		Short:        "CLI for Shipable",
		Long:         `Ship is the command-line interface for Shipable.`,
		Version:      fmt.Sprintf("%s (commit: %s, built: %s)", metadata.Version, metadata.Commit, metadata.Date),
		SilenceUsage: true, // Don't show usage on runtime errors
	}

	// Global flags
	rootCmd.PersistentFlags().String("api-url", "", "API base URL (env: SHIP_API_URL)")
	rootCmd.PersistentFlags().String("api-key", "", "API key for authentication (env: SHIP_API_KEY)")
	rootCmd.PersistentFlags().String("log-format", "text", "Log format: text or json")
	rootCmd.PersistentFlags().String("sentry-trace", "", "Sentry trace header for distributed tracing (env: SENTRY_TRACE)")
	rootCmd.PersistentFlags().String("sentry-baggage", "", "Sentry baggage header for distributed tracing (env: SENTRY_BAGGAGE)")

	// Add subcommands
	rootCmd.AddCommand(cmd_api.NewAPICmd(cliName, deps))
	rootCmd.AddCommand(cmd_apps.NewAppsCmd(cliName, deps))
	rootCmd.AddCommand(cmd_auth.NewAuthCmd(cliName, deps))
	rootCmd.AddCommand(cmd_publish.NewPublishCmd(cliName, deps))

	return rootCmd
}
