// Package cli provides the command-line interface for occb.
package cli

import (
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/spf13/cobra"
	"github.com/nilparra-dev/opencode-go-cc/internal/app"
	"github.com/nilparra-dev/opencode-go-cc/internal/config"
)

// NewServeCmd creates the serve command.
func NewServeCmd() *cobra.Command {
	var port int
	var daemonize bool

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the proxy server",
		RunE: func(cmd *cobra.Command, args []string) error {
			if daemonize {
				// Re-execute in background
				return forkIntoBackground(port)
			}

			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			if port != 0 {
				cfg.Port = port
			}

			application, err := app.NewApp(cfg, "")
			if err != nil {
				return fmt.Errorf("failed to create app: %w", err)
			}

			fmt.Printf("Starting proxy on %s:%d\n", cfg.Host, cfg.Port)
			fmt.Println("Press Ctrl+C to stop")
			return application.Start()
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
			pid, err := readPID()
			if err != nil {
				return fmt.Errorf("proxy is not running")
			}

			process, err := os.FindProcess(pid)
			if err != nil {
				_ = os.Remove(proxyPIDPath())
				return fmt.Errorf("proxy is not running")
			}

			if err := process.Signal(os.Interrupt); err != nil {
				_ = process.Kill()
			}

			_ = os.Remove(proxyPIDPath())
			fmt.Println("Proxy stopped")
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
			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			// Start proxy
			application, err := app.NewApp(cfg, "")
			if err != nil {
				return fmt.Errorf("failed to create app: %w", err)
			}

			go func() {
				_ = application.Start()
			}()

			proxyURL := fmt.Sprintf("http://%s:%d", cfg.Host, cfg.Port)
			if err := waitForProxy(proxyURL, 5*time.Second); err != nil {
				return fmt.Errorf("proxy failed to start: %w", err)
			}

			// Run claude with proxy env vars
			claudeCmd := exec.Command("claude", args...)
			claudeCmd.Stdin = os.Stdin
			claudeCmd.Stdout = os.Stdout
			claudeCmd.Stderr = os.Stderr
			claudeCmd.Env = append(os.Environ(),
				fmt.Sprintf("ANTHROPIC_BASE_URL=%s", proxyURL),
				"ANTHROPIC_AUTH_TOKEN=unused",
			)

			err = claudeCmd.Run()

			// Stop proxy
			_ = application.Stop()

			return err
		},
	}
}

// NewValidateCmd creates the validate command.
func NewValidateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "validate",
		Short: "Validate configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("invalid config: %w", err)
			}

			fmt.Println("Configuration is valid!")
			fmt.Printf("  Host:    %s\n", cfg.Host)
			fmt.Printf("  Port:    %d\n", cfg.Port)
			fmt.Printf("  API Key: %s...\n", maskString(cfg.APIKey, 8))
			fmt.Printf("  Base URL: %s\n", cfg.OpenCodeGo.BaseURL)
			fmt.Printf("  Models:  %d\n", len(cfg.Models))
			fmt.Printf("  Fallbacks: %d\n", len(cfg.Fallbacks))
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
			fmt.Println("  -----------------------------------------")
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

// maskString masks all but the first `visible` characters of a string.
func maskString(s string, visible int) string {
	if len(s) <= visible {
		return s
	}
	return s[:visible] + "..."
}
