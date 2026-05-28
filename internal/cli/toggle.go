// Package cli provides the command-line interface for occb.
package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// NewOnCmd creates the on command.
func NewOnCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "on",
		Short: "Activate OpenCode mode",
		Long:  `Starts the proxy server and configures Claude Code to use OpenCode Go models.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// TODO: Implement proxy start + settings.json update
			fmt.Println("occb on — not yet implemented")
			return nil
		},
	}
}

// NewOffCmd creates the off command.
func NewOffCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "off",
		Short: "Deactivate OpenCode mode",
		Long:  `Stops the proxy server and restores Claude Code to use Anthropic directly.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// TODO: Implement proxy stop + settings.json restore
			fmt.Println("occb off — not yet implemented")
			return nil
		},
	}
}

// NewStatusCmd creates the status command.
func NewStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show current status",
		RunE: func(cmd *cobra.Command, args []string) error {
			// TODO: Implement status check
			fmt.Println("occb status — not yet implemented")
			return nil
		},
	}
}
