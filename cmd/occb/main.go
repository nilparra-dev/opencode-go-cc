// Package main is the CLI entry point for the occb proxy server.
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/nilparra-dev/opencode-go-cc/internal/cli"
)

// Version is set at build time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	rootCmd := &cobra.Command{
		Use:   "occb",
		Short: "OpenCode Claude Bridge — Use OpenCode Go with Claude Code",
		Long: `occb is a transparent proxy that lets you use your OpenCode Go
subscription with Claude Code. It intercepts Anthropic API requests,
transforms them to OpenAI format, and forwards them to OpenCode Go.

Quick start:
  occb init    # Setup configuration
  occb on      # Activate OpenCode mode
  claude       # Launch Claude Code with OpenCode models
  occb off     # Return to normal Claude mode`,
		Version: version,
	}

	// Global flags
	var configPath string
	rootCmd.PersistentFlags().StringVarP(&configPath, "config", "c", "", "Path to config file")

	// Add subcommands
	rootCmd.AddCommand(cli.NewInitCmd())
	rootCmd.AddCommand(cli.NewOnCmd())
	rootCmd.AddCommand(cli.NewOffCmd())
	rootCmd.AddCommand(cli.NewStatusCmd())
	rootCmd.AddCommand(cli.NewServeCmd())
	rootCmd.AddCommand(cli.NewStopCmd())
	rootCmd.AddCommand(cli.NewRunCmd())
	rootCmd.AddCommand(cli.NewValidateCmd())
	rootCmd.AddCommand(cli.NewModelsCmd())
	rootCmd.AddCommand(cli.NewUpdateCmd(version))

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
