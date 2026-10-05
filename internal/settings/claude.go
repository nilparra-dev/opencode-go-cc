// Package settings manages Claude Code's settings.json for mode toggling.
package settings

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/nilparra-dev/opencode-go-cc/internal/config"
)

const (
	envAnthropicBaseURL      = "ANTHROPIC_BASE_URL"
	envAnthropicAuthToken    = "ANTHROPIC_AUTH_TOKEN"
	envAnthropicAPIKey       = "ANTHROPIC_API_KEY"
	envAnthropicModel        = "ANTHROPIC_MODEL"
	envDefaultSonnetModel    = "ANTHROPIC_DEFAULT_SONNET_MODEL"
	envDefaultOpusModel      = "ANTHROPIC_DEFAULT_OPUS_MODEL"
	envDefaultHaikuModel     = "ANTHROPIC_DEFAULT_HAIKU_MODEL"
	envSmallFastModel        = "ANTHROPIC_SMALL_FAST_MODEL"
	envDisableModelCalls     = "DISABLE_NON_ESSENTIAL_MODEL_CALLS"
	envDisableNonessential   = "CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC"
	envGatewayDiscovery      = "CLAUDE_CODE_ENABLE_GATEWAY_MODEL_DISCOVERY"

	// Dummy credentials occb writes in exclusive mode; only these are ever removed.
	placeholderAuthToken = "unused"
	placeholderAPIKey    = "occb-proxy"
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

// EnableMixedMode routes Claude Code through the proxy while keeping the user's
// own Anthropic credentials and Claude models. The proxy forwards claude-*
// requests to Anthropic untouched and serves OpenCode models, which Claude Code
// discovers from the proxy's /v1/models and adds to its /model picker.
//
// No credential variable is set, so a saved claude.ai login stays active.
func EnableMixedMode(proxyURL string) error {
	if err := EnsureOnboardingComplete(); err != nil {
		return fmt.Errorf("failed to update Claude Code onboarding state: %w", err)
	}

	s, err := Load()
	if err != nil {
		return err
	}

	// Drop whatever a previous `occb on` (possibly exclusive mode) left behind.
	clearManagedEnv(s.Env)

	s.Env[envAnthropicBaseURL] = proxyURL
	s.Env[envGatewayDiscovery] = "1"

	return s.Save()
}

// EnableOpenCodeMode updates settings.json to route all Claude Code traffic to
// OpenCode models ("exclusive" mode). It also pins Claude's default model tiers to OpenCode models so /model exposes
// the configured OpenCode options instead of Anthropic defaults.
func EnableOpenCodeMode(proxyURL string, cfg *config.Config) error {
	// First, ensure onboarding is marked complete in ~/.claude.json
	// This prevents Claude Code from ignoring ANTHROPIC_BASE_URL
	if err := EnsureOnboardingComplete(); err != nil {
		return fmt.Errorf("failed to update Claude Code onboarding state: %w", err)
	}

	s, err := Load()
	if err != nil {
		return err
	}

	clearOpenCodeModeEnv(s.Env)

	s.Env[envAnthropicBaseURL] = proxyURL
	s.Env[envGatewayDiscovery] = "1"
	// Claude Code uses the auth-token path to bootstrap custom model options from /v1/models.
	// The proxy does not validate bearer tokens, so any sentinel value works here.
	s.Env[envAnthropicAuthToken] = placeholderAuthToken

	for key, value := range OpenCodeModelEnv(cfg) {
		s.Env[key] = value
	}

	return s.Save()
}

// EnableOpenCodeModeWithAPIKey forces API key mode (for users who want to
// override their Claude.ai OAuth session).
func EnableOpenCodeModeWithAPIKey(proxyURL string, cfg *config.Config) error {
	if err := EnsureOnboardingComplete(); err != nil {
		return fmt.Errorf("failed to update Claude Code onboarding state: %w", err)
	}

	s, err := Load()
	if err != nil {
		return err
	}

	clearOpenCodeModeEnv(s.Env)

	s.Env[envAnthropicBaseURL] = proxyURL
	s.Env[envAnthropicAPIKey] = placeholderAPIKey
	s.Env[envDisableModelCalls] = "1"
	s.Env[envDisableNonessential] = "1"

	for key, value := range OpenCodeModelEnv(cfg) {
		s.Env[key] = value
	}

	return s.Save()
}

// DisableOpenCodeMode removes the proxy configuration from settings.json.
// Settings that occb did not write (the user's own API key, model pins, ...)
// are left alone.
func DisableOpenCodeMode() error {
	s, err := Load()
	if err != nil {
		return err
	}

	clearManagedEnv(s.Env)

	return s.Save()
}

// clearManagedEnv removes only what occb itself wrote.
//
// The dummy credentials mark exclusive mode; only then did occb also pin the
// model tiers and set the traffic-reduction flags, so only then are those
// removed. In mixed mode occb wrote just the base URL and the discovery flag,
// and the user's own API key, model and other settings stay untouched.
func clearManagedEnv(env map[string]string) {
	exclusive := env[envAnthropicAuthToken] == placeholderAuthToken ||
		env[envAnthropicAPIKey] == placeholderAPIKey

	if isProxyBaseURL(env[envAnthropicBaseURL]) {
		delete(env, envAnthropicBaseURL)
	}
	delete(env, envGatewayDiscovery)

	if !exclusive {
		return
	}

	clearOpenCodeModeEnv(env)
}

// isProxyBaseURL reports whether a base URL points at a local occb proxy.
func isProxyBaseURL(raw string) bool {
	if raw == "" {
		return false
	}
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	switch u.Hostname() {
	case "127.0.0.1", "localhost", "::1":
		return true
	}
	return false
}

// OpenCodeModelEnv maps Claude's built-in model tiers to a curated OpenCode picker.
// Claude only exposes a handful of custom slots in /model, so we keep the main
// default route and use the remaining slots to surface a broader set of models.
func OpenCodeModelEnv(cfg *config.Config) map[string]string {
	modelEnv := map[string]string{}
	used := make(map[string]struct{})

	slots := []struct {
		envKey     string
		candidates [][]string
	}{
		{
			envKey: envAnthropicModel,
			candidates: [][]string{
				scenarioModelCandidates(cfg, "default"),
				fallbackModelCandidates(cfg, "default"),
				{"kimi-k2.6", "deepseek-v4-pro", "qwen3.7-max", "glm-5.1"},
			},
		},
		{
			envKey: envDefaultSonnetModel,
			candidates: [][]string{
				{"deepseek-v4-pro", "mimo-v2.5-pro", "glm-5", "kimi-k2.6", "qwen3.7-max"},
				scenarioModelCandidates(cfg, "think", "default", "complex"),
				fallbackModelCandidates(cfg, "think", "default", "complex"),
			},
		},
		{
			envKey: envDefaultOpusModel,
			candidates: [][]string{
				{"qwen3.7-max", "glm-5.1", "deepseek-v4-pro", "minimax-m2.7", "mimo-v2.5-pro"},
				scenarioModelCandidates(cfg, "complex", "think", "long_context", "default"),
				fallbackModelCandidates(cfg, "complex", "think", "long_context", "default"),
			},
		},
		{
			envKey: envDefaultHaikuModel,
			candidates: [][]string{
				{"deepseek-v4-flash", "mimo-v2.5", "qwen3.5-plus", "kimi-k2.5", "glm-5"},
				scenarioModelCandidates(cfg, "background", "fast", "default"),
				fallbackModelCandidates(cfg, "background", "fast", "default"),
			},
		},
		{
			envKey: envSmallFastModel,
			candidates: [][]string{
				{"qwen3.6-plus", "minimax-m2.5", "deepseek-v4-flash", "qwen3.5-plus"},
				scenarioModelCandidates(cfg, "fast", "background", "default"),
				fallbackModelCandidates(cfg, "fast", "background", "default"),
			},
		},
	}

	for _, slot := range slots {
		if modelID := firstUnusedModelID(used, slot.candidates...); modelID != "" {
			modelEnv[slot.envKey] = modelID
			used[modelID] = struct{}{}
		}
	}

	return modelEnv
}

func firstUnusedModelID(used map[string]struct{}, candidateGroups ...[]string) string {
	seen := make(map[string]struct{})
	fallback := ""

	for _, group := range candidateGroups {
		for _, modelID := range group {
			if modelID == "" {
				continue
			}
			if _, ok := seen[modelID]; ok {
				continue
			}
			seen[modelID] = struct{}{}
			if fallback == "" {
				fallback = modelID
			}
			if _, ok := used[modelID]; !ok {
				return modelID
			}
		}
	}

	return fallback
}

func scenarioModelCandidates(cfg *config.Config, scenarios ...string) []string {
	defaultCfg := config.DefaultConfig()
	ids := make([]string, 0, len(scenarios)*2)

	appendScenarioModels := func(source *config.Config) {
		if source == nil || source.Models == nil {
			return
		}
		for _, scenario := range scenarios {
			if model, ok := source.Models[scenario]; ok && model.ModelID != "" {
				ids = append(ids, model.ModelID)
			}
		}
	}

	appendScenarioModels(cfg)
	appendScenarioModels(defaultCfg)

	return uniqueModelIDs(ids)
}

func fallbackModelCandidates(cfg *config.Config, scenarios ...string) []string {
	defaultCfg := config.DefaultConfig()
	ids := make([]string, 0, len(scenarios)*2)

	appendFallbackModels := func(source *config.Config) {
		if source == nil || source.Fallbacks == nil {
			return
		}
		for _, scenario := range scenarios {
			for _, model := range source.Fallbacks[scenario] {
				if model.ModelID != "" {
					ids = append(ids, model.ModelID)
				}
			}
		}
	}

	appendFallbackModels(cfg)
	appendFallbackModels(defaultCfg)

	return uniqueModelIDs(ids)
}

func uniqueModelIDs(ids []string) []string {
	seen := make(map[string]struct{}, len(ids))
	unique := make([]string, 0, len(ids))
	for _, id := range ids {
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		unique = append(unique, id)
	}
	return unique
}

func clearOpenCodeModeEnv(env map[string]string) {
	delete(env, envAnthropicBaseURL)
	delete(env, envAnthropicAuthToken)
	delete(env, envAnthropicAPIKey)
	delete(env, envAnthropicModel)
	delete(env, envDefaultSonnetModel)
	delete(env, envDefaultOpusModel)
	delete(env, envDefaultHaikuModel)
	delete(env, envSmallFastModel)
	delete(env, envDisableModelCalls)
	delete(env, envDisableNonessential)
	delete(env, envGatewayDiscovery)
}

func modelIDForScenario(cfg *config.Config, scenarios ...string) string {
	defaultCfg := config.DefaultConfig()
	for _, scenario := range scenarios {
		if cfg != nil && cfg.Models != nil {
			if model, ok := cfg.Models[scenario]; ok && model.ModelID != "" {
				return model.ModelID
			}
		}
		if model, ok := defaultCfg.Models[scenario]; ok && model.ModelID != "" {
			return model.ModelID
		}
	}
	return ""
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

// EnsureOnboardingComplete makes sure ~/.claude.json has hasCompletedOnboarding
// set to true. Claude Code has an onboarding gate that runs before it reads env
// vars; if onboarding is not marked complete it ignores ANTHROPIC_BASE_URL.
//
// It only adds that one key. Login data (oauthAccount, .credentials.json) and
// every other field are left exactly as they are, and the file is not rewritten
// at all when the flag is already set.
func EnsureOnboardingComplete() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	claudeJSON := filepath.Join(home, ".claude.json")

	data := make(map[string]json.RawMessage)

	raw, readErr := os.ReadFile(claudeJSON)
	switch {
	case readErr == nil:
		if err := json.Unmarshal(raw, &data); err != nil {
			// Do not clobber a file we cannot parse; the user's state lives in it.
			return fmt.Errorf("failed to parse %s: %w", claudeJSON, err)
		}
	case !os.IsNotExist(readErr):
		return fmt.Errorf("failed to read %s: %w", claudeJSON, readErr)
	}

	if string(data["hasCompletedOnboarding"]) == "true" {
		return nil
	}

	// Keep a copy of the original before touching it.
	if readErr == nil {
		backupPath := fmt.Sprintf("%s.backup.%s", claudeJSON, time.Now().Format("20060102_150405"))
		if err := os.WriteFile(backupPath, raw, 0600); err != nil {
			return fmt.Errorf("failed to back up .claude.json: %w", err)
		}
	}

	data["hasCompletedOnboarding"] = json.RawMessage("true")

	output, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal .claude.json: %w", err)
	}
	output = append(output, '\n')

	tmpPath := claudeJSON + ".tmp"
	if err := os.WriteFile(tmpPath, output, 0600); err != nil {
		return fmt.Errorf("failed to write .claude.json: %w", err)
	}
	if err := os.Rename(tmpPath, claudeJSON); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("failed to replace .claude.json: %w", err)
	}

	return nil
}
