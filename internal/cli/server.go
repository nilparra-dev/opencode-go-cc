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
			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			if port != 0 {
				cfg.Port = port
			}

			application, err := app.NewApp(cfg, proxyPIDPath())
			if err != nil {
				return fmt.Errorf("failed to create app: %w", err)
			}

			if daemonize {
				// Background mode: start server without console output
				return application.Start()
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
			fmt.Println("Available OpenCode Go models (updated May 2026):")
			fmt.Println()
			fmt.Println("  Model ID           Endpoint Type      Cost/1M input")
			fmt.Println("  ---------------------------------------------------------")
			fmt.Println("  qwen3.5-plus       Anthropic-compat   $0.20  (cheapest)")
			fmt.Println("  qwen3.6-plus       Anthropic-compat   $0.50")
			fmt.Println("  qwen3.7-max        Anthropic-compat   $2.50")
			fmt.Println("  minimax-m2.5       Anthropic-compat   $0.30  (1M context)")
			fmt.Println("  minimax-m2.7       Anthropic-compat   $0.30  (1M context)")
			fmt.Println("  glm-5              OpenAI-compat      $1.00")
			fmt.Println("  glm-5.1            OpenAI-compat      $1.40  (best quality)")
			fmt.Println("  kimi-k2.5          OpenAI-compat      $0.60")
			fmt.Println("  kimi-k2.6          OpenAI-compat      $0.95")
			fmt.Println("  deepseek-v4-pro    OpenAI-compat      $3.45")
			fmt.Println("  deepseek-v4-flash  OpenAI-compat      cheap")
			fmt.Println("  mimo-v2.5          OpenAI-compat      cheap")
			fmt.Println("  mimo-v2.5-pro      OpenAI-compat      $3.25")
			fmt.Println()
			fmt.Println("Use these model IDs in your config.yaml file.")
			fmt.Println()
			fmt.Println("Note: Models marked 'Anthropic-compat' use /v1/messages endpoint.")
			fmt.Println("      Models marked 'OpenAI-compat' use /v1/chat/completions endpoint.")
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
