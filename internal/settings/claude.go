// Package settings manages Claude Code's settings.json for mode toggling.
package settings

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// ClaudeDir returns the Claude Code settings directory.
func ClaudeDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".claude")
}

// SettingsPath returns the path to Claude Code's settings.json.
func SettingsPath() string {
	return filepath.Join(ClaudeDir(), "settings.json")
}

// Settings represents the structure of Claude Code's settings.json.
type Settings struct {
	Env map[string]string `json:"env,omitempty"`
	// Other fields are preserved but not modified
	_raw map[string]json.RawMessage
}

// Load reads Claude Code's settings.json from disk.
// If the file does not exist, it returns an empty Settings.
func Load() (*Settings, error) {
	path := SettingsPath()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Settings{Env: make(map[string]string), _raw: make(map[string]json.RawMessage)}, nil
		}
		return nil, fmt.Errorf("failed to read settings.json: %w", err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("failed to parse settings.json: %w", err)
	}

	s := &Settings{_raw: raw}

	if envRaw, ok := raw["env"]; ok {
		var env map[string]string
		if err := json.Unmarshal(envRaw, &env); err == nil {
			s.Env = env
		}
	}

	if s.Env == nil {
		s.Env = make(map[string]string)
	}

	return s, nil
}

// Save writes settings back to disk atomically with a backup.
func (s *Settings) Save() error {
	path := SettingsPath()

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("failed to create settings directory: %w", err)
	}

	// Create backup before modifying
	if _, err := os.Stat(path); err == nil {
		backupPath := fmt.Sprintf("%s.backup.%s", path, time.Now().Format("20060102_150405"))
		if data, err := os.ReadFile(path); err == nil {
			_ = os.WriteFile(backupPath, data, 0644)
		}
	}

	// Build full JSON preserving unmodified fields
	output := make(map[string]interface{})
	for k, v := range s._raw {
		output[k] = v
	}
	if len(s.Env) > 0 {
		output["env"] = s.Env
	} else {
		delete(output, "env")
	}

	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal settings: %w", err)
	}
	data = append(data, '\n')

	// Atomic write
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write temp settings file: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("failed to replace settings file: %w", err)
	}

	return nil
}

// EnableOpenCodeMode updates settings.json to route Claude Code through the proxy.
// It also ensures ~/.claude.json has hasCompletedOnboarding set to true,
// which is required for Claude Code to respect ANTHROPIC_BASE_URL.
func EnableOpenCodeMode(proxyURL string) error {
	// First, ensure onboarding is marked complete in ~/.claude.json
	// This prevents Claude Code from ignoring ANTHROPIC_BASE_URL
	if err := EnsureOnboardingComplete(); err != nil {
		return fmt.Errorf("failed to update Claude Code onboarding state: %w", err)
	}

	s, err := Load()
	if err != nil {
		return err
	}

	s.Env["ANTHROPIC_BASE_URL"] = proxyURL
	s.Env["ANTHROPIC_API_KEY"] = "occb-proxy"
	s.Env["DISABLE_NON_ESSENTIAL_MODEL_CALLS"] = "1"
	s.Env["CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC"] = "1"
	// Remove ANTHROPIC_AUTH_TOKEN to avoid auth conflict with API key mode
	delete(s.Env, "ANTHROPIC_AUTH_TOKEN")

	return s.Save()
}

// EnableOpenCodeModeWithAPIKey forces API key mode (for users who want to
// override their Claude.ai OAuth session).
func EnableOpenCodeModeWithAPIKey(proxyURL string) error {
	if err := EnsureOnboardingComplete(); err != nil {
		return fmt.Errorf("failed to update Claude Code onboarding state: %w", err)
	}

	s, err := Load()
	if err != nil {
		return err
	}

	s.Env["ANTHROPIC_BASE_URL"] = proxyURL
	s.Env["ANTHROPIC_API_KEY"] = "occb-proxy"
	s.Env["DISABLE_NON_ESSENTIAL_MODEL_CALLS"] = "1"
	s.Env["CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC"] = "1"
	delete(s.Env, "ANTHROPIC_AUTH_TOKEN")

	return s.Save()
}

// DisableOpenCodeMode removes the proxy configuration from settings.json.
func DisableOpenCodeMode() error {
	s, err := Load()
	if err != nil {
		return err
	}

	delete(s.Env, "ANTHROPIC_BASE_URL")
	delete(s.Env, "ANTHROPIC_AUTH_TOKEN")
	delete(s.Env, "ANTHROPIC_API_KEY")
	delete(s.Env, "DISABLE_NON_ESSENTIAL_MODEL_CALLS")
	delete(s.Env, "CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC")

	return s.Save()
}

// IsOpenCodeModeEnabled checks if Claude Code is configured to use the proxy.
func IsOpenCodeModeEnabled() (bool, error) {
	s, err := Load()
	if err != nil {
		return false, err
	}

	_, hasBaseURL := s.Env["ANTHROPIC_BASE_URL"]
	return hasBaseURL, nil
}

// IsClaudeAuthenticated checks if Claude Code has an active OAuth session.
// It checks ~/.claude.json for session/token data, not just file existence.
func IsClaudeAuthenticated() bool {
	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}
	claudeJSON := filepath.Join(home, ".claude.json")
	data, err := os.ReadFile(claudeJSON)
	if err != nil {
		return false
	}

	// Check if the file contains session/auth data
	content := string(data)
	return containsAny(content, []string{
		`"session"`,
		`"token"`,
		`"accessToken"`,
		`"refreshToken"`,
		`"account"`,
		`"oauthAccount"`,
		`"user"`,
		`"email"`,
		`"organization"`,
		`"billingType"`,
	})
}

// containsAny returns true if s contains any of the substrings.
func containsAny(s string, subs []string) bool {
	for _, sub := range subs {
		if contains(s, sub) {
			return true
		}
	}
	return false
}

// contains checks if s contains sub.
func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsHelper(s, sub))
}

func containsHelper(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// EnsureOnboardingComplete ensures ~/.claude.json has hasCompletedOnboarding set to true.
// Claude Code has an onboarding gate that runs before reading env vars. If onboarding
// is not marked complete, it ignores ANTHROPIC_BASE_URL and forces OAuth login.
func EnsureOnboardingComplete() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	claudeJSON := filepath.Join(home, ".claude.json")

	var data map[string]interface{}

	if raw, err := os.ReadFile(claudeJSON); err == nil {
		// File exists, parse it
		if err := json.Unmarshal(raw, &data); err != nil {
			// If it's not valid JSON, overwrite with minimal config
			data = make(map[string]interface{})
		}
	} else {
		// File doesn't exist, start fresh
		data = make(map[string]interface{})
	}

	// Ensure hasCompletedOnboarding is set
	data["hasCompletedOnboarding"] = true

	output, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal .claude.json: %w", err)
	}
	output = append(output, '\n')

	if err := os.WriteFile(claudeJSON, output, 0644); err != nil {
		return fmt.Errorf("failed to write .claude.json: %w", err)
	}

	return nil
}
