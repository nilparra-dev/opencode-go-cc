package settings

import (
	"strings"
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

func setupClaudeHome(t *testing.T) (home string) {
	t.Helper()
	home = t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	if err := os.MkdirAll(filepath.Join(home, ".claude"), 0755); err != nil {
		t.Fatal(err)
	}
	return home
}

func TestEnsureOnboardingCompleteKeepsLoginAndOtherState(t *testing.T) {
	home := setupClaudeHome(t)

	claudeJSON := filepath.Join(home, ".claude.json")
	credentials := filepath.Join(home, ".claude", ".credentials.json")
	original := `{"oauthAccount":{"emailAddress":"me@example.com"},"numStartups":42,"bigId":12345678901234567890}`
	if err := os.WriteFile(claudeJSON, []byte(original), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(credentials, []byte(`{"token":"secret"}`), 0600); err != nil {
		t.Fatal(err)
	}

	if err := EnsureOnboardingComplete(); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(claudeJSON)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"oauthAccount"`, `me@example.com`, `"numStartups": 42`, `12345678901234567890`, `"hasCompletedOnboarding": true`} {
		if !strings.Contains(string(data), want) {
			t.Errorf(".claude.json lost %q:\n%s", want, data)
		}
	}
	if _, err := os.Stat(credentials); err != nil {
		t.Errorf("credentials file must not be removed: %v", err)
	}

	backups, _ := filepath.Glob(claudeJSON + ".backup.*")
	if len(backups) != 1 {
		t.Fatalf("expected exactly one backup of the original, got %v", backups)
	}
}

func TestEnsureOnboardingCompleteDoesNotRewriteWhenAlreadySet(t *testing.T) {
	home := setupClaudeHome(t)

	claudeJSON := filepath.Join(home, ".claude.json")
	original := `{"hasCompletedOnboarding":true,"oauthAccount":{"emailAddress":"me@example.com"}}`
	if err := os.WriteFile(claudeJSON, []byte(original), 0600); err != nil {
		t.Fatal(err)
	}

	if err := EnsureOnboardingComplete(); err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(claudeJSON)
	if string(data) != original {
		t.Fatalf("file was rewritten:\n%s", data)
	}
	if backups, _ := filepath.Glob(claudeJSON + ".backup.*"); len(backups) != 0 {
		t.Fatalf("no backup expected when nothing changes, got %v", backups)
	}
}

func TestEnsureOnboardingCompleteRefusesToOverwriteUnparseableFile(t *testing.T) {
	home := setupClaudeHome(t)

	claudeJSON := filepath.Join(home, ".claude.json")
	if err := os.WriteFile(claudeJSON, []byte(`{not json`), 0600); err != nil {
		t.Fatal(err)
	}

	if err := EnsureOnboardingComplete(); err == nil {
		t.Fatal("expected an error for an unparseable .claude.json")
	}
	if data, _ := os.ReadFile(claudeJSON); string(data) != `{not json` {
		t.Fatalf("unparseable file was modified: %s", data)
	}
}

func TestEnableMixedModeKeepsUserCredentialsAndModels(t *testing.T) {
	home := setupClaudeHome(t)

	settingsPath := filepath.Join(home, ".claude", "settings.json")
	initial := `{"model":"opus","env":{"ANTHROPIC_API_KEY":"sk-user","ANTHROPIC_MODEL":"claude-opus-5-5"}}`
	if err := os.WriteFile(settingsPath, []byte(initial), 0644); err != nil {
		t.Fatal(err)
	}

	if err := EnableMixedMode("http://127.0.0.1:3456"); err != nil {
		t.Fatal(err)
	}

	s, err := Load()
	if err != nil {
		t.Fatal(err)
	}

	if s.Env["ANTHROPIC_BASE_URL"] != "http://127.0.0.1:3456" || s.Env["CLAUDE_CODE_ENABLE_GATEWAY_MODEL_DISCOVERY"] != "1" {
		t.Fatalf("proxy not configured: %v", s.Env)
	}
	if s.Env["ANTHROPIC_API_KEY"] != "sk-user" || s.Env["ANTHROPIC_MODEL"] != "claude-opus-5-5" {
		t.Fatalf("user's own settings were removed: %v", s.Env)
	}
	if _, ok := s.Env["ANTHROPIC_AUTH_TOKEN"]; ok {
		t.Fatal("mixed mode must not set a credential variable (it would replace the claude.ai login)")
	}
	if data, _ := os.ReadFile(settingsPath); !strings.Contains(string(data), `"model": "opus"`) {
		t.Fatalf("unrelated settings fields were lost:\n%s", data)
	}
}

func TestDisableOpenCodeModeRestoresAndKeepsUserSettings(t *testing.T) {
	home := setupClaudeHome(t)

	settingsPath := filepath.Join(home, ".claude", "settings.json")
	if err := os.WriteFile(settingsPath, []byte(`{"env":{"ANTHROPIC_API_KEY":"sk-user","FOO":"bar"}}`), 0644); err != nil {
		t.Fatal(err)
	}

	if err := EnableMixedMode("http://127.0.0.1:3456"); err != nil {
		t.Fatal(err)
	}
	if err := DisableOpenCodeMode(); err != nil {
		t.Fatal(err)
	}

	s, _ := Load()
	if _, ok := s.Env["ANTHROPIC_BASE_URL"]; ok {
		t.Fatal("base URL should be removed")
	}
	if s.Env["ANTHROPIC_API_KEY"] != "sk-user" || s.Env["FOO"] != "bar" {
		t.Fatalf("user settings must survive `occb off`: %v", s.Env)
	}
}

func TestDisableOpenCodeModeAfterExclusiveRemovesPinnedModels(t *testing.T) {
	setupClaudeHome(t)
	cfg := &config.Config{}

	if err := EnableOpenCodeMode("http://127.0.0.1:3456", cfg); err != nil {
		t.Fatal(err)
	}
	if err := DisableOpenCodeMode(); err != nil {
		t.Fatal(err)
	}

	s, _ := Load()
	if len(s.Env) != 0 {
		t.Fatalf("exclusive-mode settings should all be removed, left: %v", s.Env)
	}
}

func TestDisableOpenCodeModeKeepsUsersOwnGatewayModels(t *testing.T) {
	home := setupClaudeHome(t)

	settingsPath := filepath.Join(home, ".claude", "settings.json")
	if err := os.WriteFile(settingsPath, []byte(`{"env":{"ANTHROPIC_BASE_URL":"https://gateway.corp.example","ANTHROPIC_MODEL":"corp-model"}}`), 0644); err != nil {
		t.Fatal(err)
	}

	if err := DisableOpenCodeMode(); err != nil {
		t.Fatal(err)
	}

	s, _ := Load()
	if s.Env["ANTHROPIC_BASE_URL"] != "https://gateway.corp.example" || s.Env["ANTHROPIC_MODEL"] != "corp-model" {
		t.Fatalf("a non-occb gateway config must be left alone: %v", s.Env)
	}
}
