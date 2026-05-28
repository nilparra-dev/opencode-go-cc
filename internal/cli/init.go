// Package cli provides the command-line interface for occb.
package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/nilparra-dev/opencode-go-cc/internal/config"
)

// NewInitCmd creates the init command.
func NewInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Create default configuration file",
		RunE: func(cmd *cobra.Command, args []string) error {
			configDir := config.ConfigDir()
			configPath := filepath.Join(configDir, "config.yaml")

			// Check if config already exists
			if _, err := os.Stat(configPath); err == nil {
				fmt.Printf("Config already exists at %s\n", configPath)
				fmt.Println("Edit the file to update your configuration.")
				return nil
			}

			if err := os.MkdirAll(configDir, 0755); err != nil {
				return fmt.Errorf("failed to create config directory: %w", err)
			}

			// Write default config
			defaultConfig := getDefaultConfigYAML()
			if err := os.WriteFile(configPath, []byte(defaultConfig), 0644); err != nil {
				return fmt.Errorf("failed to write config file: %w", err)
			}

			fmt.Printf("Created default config at %s\n", configPath)
			fmt.Println("Add your OpenCode Go API key to the file or set OCB_API_KEY.")
			return nil
		},
	}
}

func getDefaultConfigYAML() string {
	return `api_key: "${OCB_API_KEY}"
host: "127.0.0.1"
port: 3456
hot_reload: false
enable_streaming_scenario_routing: false
respect_requested_model: false

models:
  default:
    provider: "opencode-go"
    model_id: "kimi-k2.6"
    temperature: 0.7
    max_tokens: 4096
  background:
    provider: "opencode-go"
    model_id: "qwen3.5-plus"
    temperature: 0.5
    max_tokens: 2048
  think:
    provider: "opencode-go"
    model_id: "glm-5"
    temperature: 0.7
    max_tokens: 8192
  complex:
    provider: "opencode-go"
    model_id: "glm-5.1"
    temperature: 0.7
    max_tokens: 4096
  long_context:
    provider: "opencode-go"
    model_id: "minimax-m2.5"
    temperature: 0.7
    max_tokens: 16384
    context_threshold: 80000
  fast:
    provider: "opencode-go"
    model_id: "qwen3.6-plus"
    temperature: 0.7
    max_tokens: 4096

fallbacks:
  default:
    - provider: "opencode-go"
      model_id: "mimo-v2-pro"
    - provider: "opencode-go"
      model_id: "qwen3.6-plus"
  background:
    - provider: "opencode-go"
      model_id: "qwen3.6-plus"
    - provider: "opencode-go"
      model_id: "minimax-m2.5"
  think:
    - provider: "opencode-go"
      model_id: "kimi-k2.6"
    - provider: "opencode-go"
      model_id: "mimo-v2-pro"
  complex:
    - provider: "opencode-go"
      model_id: "glm-5"
    - provider: "opencode-go"
      model_id: "kimi-k2.6"
  long_context:
    - provider: "opencode-go"
      model_id: "minimax-m2.7"
    - provider: "opencode-go"
      model_id: "kimi-k2.6"
  fast:
    - provider: "opencode-go"
      model_id: "qwen3.5-plus"
    - provider: "opencode-go"
      model_id: "minimax-m2.5"

opencode_go:
  base_url: "https://opencode.ai/zen/go/v1/chat/completions"
  anthropic_base_url: "https://opencode.ai/zen/go/v1/messages"
  timeout_ms: 300000

logging:
  level: "info"
  requests: true
`
}
