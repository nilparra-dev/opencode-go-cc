package settings

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/nilparra-dev/opencode-go-cc/internal/config"
)

func TestEnableOpenCodeModeUsesAuthTokenBootstrap(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	settingsDir := filepath.Join(home, ".claude")
	if err := os.MkdirAll(settingsDir, 0755); err != nil {
		t.Fatalf("failed to create settings dir: %v", err)
	}

	settingsPath := filepath.Join(settingsDir, "settings.json")
	initial := []byte(`{
	  "env": {
	    "ANTHROPIC_API_KEY": "stale",
	    "DISABLE_NON_ESSENTIAL_MODEL_CALLS": "1",
	    "CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC": "1"
	  }
	}`)
	if err := os.WriteFile(settingsPath, initial, 0644); err != nil {
		t.Fatalf("failed to seed settings.json: %v", err)
	}

	cfg := &config.Config{
		Models: map[string]config.ModelConfig{
			"default":    {ModelID: "kimi-k2.6"},
			"background": {ModelID: "qwen3.5-plus"},
			"complex":    {ModelID: "glm-5.1"},
			"fast":       {ModelID: "qwen3.6-plus"},
		},
	}

	if err := EnableOpenCodeMode("http://127.0.0.1:3456", cfg); err != nil {
		t.Fatalf("EnableOpenCodeMode returned error: %v", err)
	}

	settings, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if got := settings.Env["ANTHROPIC_BASE_URL"]; got != "http://127.0.0.1:3456" {
		t.Fatalf("unexpected ANTHROPIC_BASE_URL: got %q", got)
	}
	if got := settings.Env["ANTHROPIC_AUTH_TOKEN"]; got != "unused" {
		t.Fatalf("unexpected ANTHROPIC_AUTH_TOKEN: got %q", got)
	}
	if got := settings.Env["ANTHROPIC_MODEL"]; got != "kimi-k2.6" {
		t.Fatalf("unexpected ANTHROPIC_MODEL: got %q", got)
	}
	if got := settings.Env["ANTHROPIC_DEFAULT_SONNET_MODEL"]; got != "deepseek-v4-pro" {
		t.Fatalf("unexpected ANTHROPIC_DEFAULT_SONNET_MODEL: got %q", got)
	}
	if got := settings.Env["ANTHROPIC_DEFAULT_OPUS_MODEL"]; got != "qwen3.7-max" {
		t.Fatalf("unexpected ANTHROPIC_DEFAULT_OPUS_MODEL: got %q", got)
	}
	if got := settings.Env["ANTHROPIC_DEFAULT_HAIKU_MODEL"]; got != "deepseek-v4-flash" {
		t.Fatalf("unexpected ANTHROPIC_DEFAULT_HAIKU_MODEL: got %q", got)
	}
	if got := settings.Env["ANTHROPIC_SMALL_FAST_MODEL"]; got != "qwen3.6-plus" {
		t.Fatalf("unexpected ANTHROPIC_SMALL_FAST_MODEL: got %q", got)
	}
	if _, ok := settings.Env["ANTHROPIC_API_KEY"]; ok {
		t.Fatalf("ANTHROPIC_API_KEY should be removed")
	}
	if _, ok := settings.Env["DISABLE_NON_ESSENTIAL_MODEL_CALLS"]; ok {
		t.Fatalf("DISABLE_NON_ESSENTIAL_MODEL_CALLS should be removed")
	}
	if _, ok := settings.Env["CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC"]; ok {
		t.Fatalf("CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC should be removed")
	}

	claudeJSON := filepath.Join(home, ".claude.json")
	if _, err := os.Stat(claudeJSON); err != nil {
		t.Fatalf("expected %s to exist: %v", claudeJSON, err)
	}
}

func TestOpenCodeModelEnvFallsBackToDefaults(t *testing.T) {
	t.Parallel()

	modelEnv := OpenCodeModelEnv(&config.Config{})

	if modelEnv["ANTHROPIC_MODEL"] != "kimi-k2.6" {
		t.Fatalf("unexpected default ANTHROPIC_MODEL: %q", modelEnv["ANTHROPIC_MODEL"])
	}
	if modelEnv["ANTHROPIC_DEFAULT_SONNET_MODEL"] != "deepseek-v4-pro" {
		t.Fatalf("unexpected default ANTHROPIC_DEFAULT_SONNET_MODEL: %q", modelEnv["ANTHROPIC_DEFAULT_SONNET_MODEL"])
	}
	if modelEnv["ANTHROPIC_DEFAULT_OPUS_MODEL"] != "qwen3.7-max" {
		t.Fatalf("unexpected default ANTHROPIC_DEFAULT_OPUS_MODEL: %q", modelEnv["ANTHROPIC_DEFAULT_OPUS_MODEL"])
	}
	if modelEnv["ANTHROPIC_DEFAULT_HAIKU_MODEL"] != "deepseek-v4-flash" {
		t.Fatalf("unexpected default ANTHROPIC_DEFAULT_HAIKU_MODEL: %q", modelEnv["ANTHROPIC_DEFAULT_HAIKU_MODEL"])
	}
	if modelEnv["ANTHROPIC_SMALL_FAST_MODEL"] != "qwen3.6-plus" {
		t.Fatalf("unexpected default ANTHROPIC_SMALL_FAST_MODEL: %q", modelEnv["ANTHROPIC_SMALL_FAST_MODEL"])
	}
}