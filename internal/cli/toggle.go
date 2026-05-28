// Package cli provides the command-line interface for occb.
package cli

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/spf13/cobra"
	"github.com/nilparra-dev/opencode-go-cc/internal/app"
	"github.com/nilparra-dev/opencode-go-cc/internal/config"
	"github.com/nilparra-dev/opencode-go-cc/internal/settings"
)

const proxyPIDFile = "proxy.pid"

func proxyPIDPath() string {
	return filepath.Join(config.ConfigDir(), proxyPIDFile)
}

// NewOnCmd creates the on command.
func NewOnCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "on",
		Short: "Activate OpenCode mode",
		Long:  `Starts the proxy server and configures Claude Code to use OpenCode Go models.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Check if already running
			if pid, err := readPID(); err == nil && isProcessRunning(pid) {
				fmt.Printf("Proxy is already running (PID %d)\n", pid)
				fmt.Println("Run 'occb off' first if you want to restart.")
				return nil
			}

			// Load config
			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			// Start proxy in background
			fmt.Println("Starting proxy server...")
			application, err := app.NewApp(cfg, proxyPIDPath())
			if err != nil {
				return fmt.Errorf("failed to create app: %w", err)
			}

			go func() {
				_ = application.Start()
			}()

			// Wait for proxy to be ready
			proxyURL := fmt.Sprintf("http://%s:%d", cfg.Host, cfg.Port)
			if err := waitForProxy(proxyURL, 5*time.Second); err != nil {
				return fmt.Errorf("proxy failed to start: %w", err)
			}

			// Update Claude Code settings
			if err := settings.EnableOpenCodeMode(proxyURL); err != nil {
				return fmt.Errorf("failed to update Claude Code settings: %w", err)
			}

			fmt.Println()
			fmt.Println("✓ OpenCode mode activated")
			fmt.Printf("  Proxy: %s\n", proxyURL)
			fmt.Println("  Run 'claude' to start coding with OpenCode models")
			fmt.Println()
			fmt.Println("Run 'occb off' to return to normal Claude mode")
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
			// Stop proxy
			pid, err := readPID()
			if err == nil {
				if isProcessRunning(pid) {
					process, err := os.FindProcess(pid)
					if err == nil {
						_ = process.Signal(os.Interrupt)
						// Wait a bit for graceful shutdown
						time.Sleep(500 * time.Millisecond)
						if isProcessRunning(pid) {
							_ = process.Kill()
						}
					}
				}
				_ = os.Remove(proxyPIDPath())
			}

			// Restore Claude Code settings
			if err := settings.DisableOpenCodeMode(); err != nil {
				return fmt.Errorf("failed to restore Claude Code settings: %w", err)
			}

			fmt.Println()
			fmt.Println("✓ OpenCode mode deactivated")
			fmt.Println("  Claude Code is now using Anthropic directly")
			fmt.Println()
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
			fmt.Println()
			fmt.Println("OpenCode Claude Bridge (occb)")
			fmt.Println()

			// Proxy status — use health check as primary source of truth
			proxyRunning := isProxyRunning()
			if proxyRunning {
				pid, pidErr := readPID()
				if pidErr == nil {
					fmt.Printf("  Proxy:   running (PID %d)\n", pid)
				} else {
					fmt.Println("  Proxy:   running")
				}
			} else {
				fmt.Println("  Proxy:   stopped")
			}

			// Mode status
			enabled, err := settings.IsOpenCodeModeEnabled()
			if err != nil {
				fmt.Printf("  Mode:    unknown (%v)\n", err)
			} else if enabled {
				fmt.Println("  Mode:    OpenCode (via proxy)")
			} else {
				fmt.Println("  Mode:    Anthropic (direct)")
			}

			fmt.Println()
			return nil
		},
	}
}

// readPID reads the proxy PID from file.
func readPID() (int, error) {
	data, err := os.ReadFile(proxyPIDPath())
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(string(data))
}

// isProxyRunning checks if the proxy is responding to health checks.
func isProxyRunning() bool {
	client := &http.Client{Timeout: 500 * time.Millisecond}
	resp, err := client.Get("http://127.0.0.1:3456/health")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// isProcessRunning checks if the proxy process is actually responding.
func isProcessRunning(pid int) bool {
	return isProxyRunning()
}

// waitForProxy polls the proxy health endpoint until ready or timeout.
func waitForProxy(url string, timeout time.Duration) error {
	client := &http.Client{Timeout: 1 * time.Second}
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		resp, err := client.Get(url + "/health")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
		time.Sleep(200 * time.Millisecond)
	}

	return fmt.Errorf("proxy did not become ready within %v", timeout)
}
