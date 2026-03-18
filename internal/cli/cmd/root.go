// Package cmd contains all CLI commands and their implementation.
package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	cmd_apps "github.com/kula-app/ship/internal/cli/cmd/apps"
	cmd_auth "github.com/kula-app/ship/internal/cli/cmd/auth"
	cmd_publish "github.com/kula-app/ship/internal/cli/cmd/publish"
)

// BuildMetadata holds build-time version information.
type BuildMetadata struct {
	Version string
	Commit  string
	Date    string
}

// NewRootCommand creates the fully-wired root command for the ship CLI.
func NewRootCommand(cliName string, metadata BuildMetadata) *cobra.Command {
	rootCmd := &cobra.Command{
		Use:          cliName,
		Short:        "CLI for Shipable",
		Long:         `Ship is the command-line interface for Shipable.`,
		Version:      fmt.Sprintf("%s (commit: %s, built: %s)", metadata.Version, metadata.Commit, metadata.Date),
		SilenceUsage: true,
	}

	// Global flags
	rootCmd.PersistentFlags().String("log-format", "text", "Log format: text or json")

	// Add subcommands
	rootCmd.AddCommand(cmd_apps.NewAppsCmd(cliName))
	rootCmd.AddCommand(cmd_auth.NewAuthCmd(cliName))
	rootCmd.AddCommand(cmd_publish.NewPublishCmd(cliName))

	return rootCmd
}
