// Package cli provides the command-line interface for occb.
package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// NewServeCmd creates the serve command.
func NewServeCmd() *cobra.Command {
	var port int
	var daemonize bool

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the proxy server",
		RunE: func(cmd *cobra.Command, args []string) error {
			// TODO: Implement server start
			fmt.Printf("occb serve — not yet implemented (port=%d, daemonize=%v)\n", port, daemonize)
			return nil
		},
	}

	cmd.Flags().IntVarP(&port, "port", "p", 0, "Override listen port")
	cmd.Flags().BoolVar(&daemonize, "_daemonize", false, "Internal use only")
	_ = cmd.Flags().MarkHidden("_daemonize")

	return cmd
}

// NewStopCmd creates the stop command.
func NewStopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: "Stop the proxy server",
		RunE: func(cmd *cobra.Command, args []string) error {
			// TODO: Implement proxy stop
			fmt.Println("occb stop — not yet implemented")
			return nil
		},
	}
}

// NewRunCmd creates the run command.
func NewRunCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "run [args...]",
		Short: "Run Claude Code with a temporary proxy",
		Long:  `Starts the proxy, launches Claude Code with the given arguments, and stops the proxy when Claude exits.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// TODO: Implement temporary proxy + claude execution
			fmt.Println("occb run — not yet implemented")
			return nil
		},
	}
}

// NewValidateCmd creates the validate command.
func NewValidateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "validate",
		Short: "Validate configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			// TODO: Implement config validation
			fmt.Println("occb validate — not yet implemented")
			return nil
		},
	}
}

// NewModelsCmd creates the models command.
func NewModelsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "models",
		Short: "List available OpenCode Go models",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("Available OpenCode Go models:")
			fmt.Println()
			fmt.Println("  Model ID           Endpoint Type")
			fmt.Println("  ─────────────────────────────────────────")
			fmt.Println("  glm-5.1            OpenAI-compatible")
			fmt.Println("  glm-5              OpenAI-compatible")
			fmt.Println("  kimi-k2.6          OpenAI-compatible")
			fmt.Println("  kimi-k2.5          OpenAI-compatible")
			fmt.Println("  mimo-v2.5-pro      OpenAI-compatible")
			fmt.Println("  mimo-v2.5          OpenAI-compatible")
			fmt.Println("  mimo-v2-pro        OpenAI-compatible")
			fmt.Println("  mimo-v2-omni       OpenAI-compatible")
			fmt.Println("  minimax-m2.7       Anthropic-compatible")
			fmt.Println("  minimax-m2.5       Anthropic-compatible")
			fmt.Println("  deepseek-v4-pro    OpenAI-compatible")
			fmt.Println("  deepseek-v4-flash  OpenAI-compatible")
			fmt.Println("  qwen3.6-plus       OpenAI-compatible")
			fmt.Println("  qwen3.5-plus       OpenAI-compatible")
			fmt.Println()
			fmt.Println("Use these model IDs in your config.yaml file.")
		},
	}
}
