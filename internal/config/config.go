// Package config handles application configuration loading and validation.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/spf13/viper"
)

// Config holds the complete application configuration.
type Config struct {
	APIKey                         string                   `mapstructure:"api_key"`
	Host                           string                   `mapstructure:"host"`
	Port                           int                      `mapstructure:"port"`
	HotReload                      bool                     `mapstructure:"hot_reload"`
	EnableStreamingScenarioRouting bool                     `mapstructure:"enable_streaming_scenario_routing"`
	RespectRequestedModel          bool                     `mapstructure:"respect_requested_model"`
	Models                         map[string]ModelConfig   `mapstructure:"models"`
	Fallbacks                      map[string][]ModelConfig `mapstructure:"fallbacks"`
	OpenCodeGo                     OpenCodeGoConfig         `mapstructure:"opencode_go"`
	Logging                        LoggingConfig            `mapstructure:"logging"`
}

// ModelConfig defines routing rules for a specific model.
type ModelConfig struct {
	Provider         string          `mapstructure:"provider"`
	ModelID          string          `mapstructure:"model_id"`
	Temperature      float64         `mapstructure:"temperature"`
	MaxTokens        int             `mapstructure:"max_tokens"`
	ContextThreshold int             `mapstructure:"context_threshold"`
	ReasoningEffort  string          `mapstructure:"reasoning_effort"`
	Thinking         json.RawMessage `mapstructure:"thinking,omitempty"`
}

// OpenCodeGoConfig holds the upstream OpenCode Go API settings.
type OpenCodeGoConfig struct {
	BaseURL          string `mapstructure:"base_url"`
	AnthropicBaseURL string `mapstructure:"anthropic_base_url"`
	TimeoutMs        int    `mapstructure:"timeout_ms"`
}

// LoggingConfig controls application logging behavior.
type LoggingConfig struct {
	Level    string `mapstructure:"level"`
	Requests bool   `mapstructure:"requests"`
}

// AtomicConfig provides thread-safe access to the configuration.
type AtomicConfig struct {
	mu     sync.RWMutex
	config *Config
	path   string
}

// NewAtomicConfig creates a new atomic configuration wrapper.
func NewAtomicConfig(cfg *Config, path string) *AtomicConfig {
	return &AtomicConfig{config: cfg, path: path}
}

// Get returns the current configuration safely.
func (a *AtomicConfig) Get() *Config {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.config
}

// Swap updates the configuration atomically.
func (a *AtomicConfig) Swap(cfg *Config) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.config = cfg
}

// DefaultConfig returns the default configuration.
func DefaultConfig() *Config {
	return &Config{
		Host:  "127.0.0.1",
		Port:  3456,
		Models: map[string]ModelConfig{
			"default": {
				Provider:    "opencode-go",
				ModelID:     "kimi-k2.6",
				Temperature: 0.7,
				MaxTokens:   4096,
			},
			"background": {
				Provider:    "opencode-go",
				ModelID:     "qwen3.5-plus",
				Temperature: 0.5,
				MaxTokens:   2048,
			},
			"think": {
				Provider:    "opencode-go",
				ModelID:     "glm-5",
				Temperature: 0.7,
				MaxTokens:   8192,
			},
			"complex": {
				Provider:    "opencode-go",
				ModelID:     "glm-5.1",
				Temperature: 0.7,
				MaxTokens:   4096,
			},
			"long_context": {
				Provider:         "opencode-go",
				ModelID:          "minimax-m2.5",
				Temperature:      0.7,
				MaxTokens:        16384,
				ContextThreshold: 80000,
			},
			"fast": {
				Provider:    "opencode-go",
				ModelID:     "qwen3.6-plus",
				Temperature: 0.7,
				MaxTokens:   4096,
			},
		},
		Fallbacks: map[string][]ModelConfig{
			"default": {
				{Provider: "opencode-go", ModelID: "mimo-v2-pro"},
				{Provider: "opencode-go", ModelID: "qwen3.6-plus"},
			},
			"background": {
				{Provider: "opencode-go", ModelID: "qwen3.6-plus"},
				{Provider: "opencode-go", ModelID: "minimax-m2.5"},
			},
			"think": {
				{Provider: "opencode-go", ModelID: "kimi-k2.6"},
				{Provider: "opencode-go", ModelID: "mimo-v2-pro"},
			},
			"complex": {
				{Provider: "opencode-go", ModelID: "glm-5"},
				{Provider: "opencode-go", ModelID: "kimi-k2.6"},
			},
			"long_context": {
				{Provider: "opencode-go", ModelID: "minimax-m2.7"},
				{Provider: "opencode-go", ModelID: "kimi-k2.6"},
			},
			"fast": {
				{Provider: "opencode-go", ModelID: "qwen3.5-plus"},
				{Provider: "opencode-go", ModelID: "minimax-m2.5"},
			},
		},
		OpenCodeGo: OpenCodeGoConfig{
			BaseURL:          "https://opencode.ai/zen/go/v1/chat/completions",
			AnthropicBaseURL: "https://opencode.ai/zen/go/v1/messages",
			TimeoutMs:        300000,
		},
		Logging: LoggingConfig{
			Level:    "info",
			Requests: true,
		},
	}
}

// Load reads configuration from file, environment variables, and defaults.
func Load() (*Config, error) {
	v := viper.New()

	// Set defaults
	v.SetDefault("host", "127.0.0.1")
	v.SetDefault("port", 3456)
	v.SetDefault("hot_reload", false)
	v.SetDefault("enable_streaming_scenario_routing", false)
	v.SetDefault("respect_requested_model", false)
	v.SetDefault("opencode_go.base_url", "https://opencode.ai/zen/go/v1/chat/completions")
	v.SetDefault("opencode_go.anthropic_base_url", "https://opencode.ai/zen/go/v1/messages")
	v.SetDefault("opencode_go.timeout_ms", 300000)
	v.SetDefault("logging.level", "info")
	v.SetDefault("logging.requests", true)

	// Config file
	v.SetConfigName("config")
	v.SetConfigType("yaml")

	configDir := ConfigDir()
	v.AddConfigPath(configDir)

	// Environment variables
	v.SetEnvPrefix("OCB")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Allow override via OCB_CONFIG env var
	if configPath := os.Getenv("OCB_CONFIG"); configPath != "" {
		v.SetConfigFile(configPath)
	}

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("failed to read config: %w", err)
		}
		// Config file not found; use defaults and env vars
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &cfg, nil
}

// ConfigDir returns the default configuration directory.
func ConfigDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".config", "occb")
}

// ResolveConfigPath returns the resolved path to the config file.
func ResolveConfigPath() string {
	if p := os.Getenv("OCB_CONFIG"); p != "" {
		return p
	}
	return filepath.Join(ConfigDir(), "config.yaml")
}
