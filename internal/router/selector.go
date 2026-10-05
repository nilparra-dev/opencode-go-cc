// Package router handles model selection based on request scenarios.
package router

import (
	"regexp"
	"strings"

	"github.com/nilparra-dev/opencode-go-cc/internal/config"
	"github.com/nilparra-dev/opencode-go-cc/pkg/types"
)

// Scenario represents the type of request context.
type Scenario string

const (
	ScenarioDefault     Scenario = "default"
	ScenarioThink       Scenario = "think"
	ScenarioComplex     Scenario = "complex"
	ScenarioLongContext Scenario = "long_context"
	ScenarioBackground  Scenario = "background"
	ScenarioFast        Scenario = "fast"
)

// Result contains the selected model and fallback chain.
type Result struct {
	Primary   config.ModelConfig
	Fallbacks []config.ModelConfig
	Scenario  Scenario
}

// GetModelChain returns the full chain of models to try.
func (r *Result) GetModelChain() []config.ModelConfig {
	chain := []config.ModelConfig{r.Primary}
	chain = append(chain, r.Fallbacks...)
	return chain
}

// ModelSelector handles model selection based on scenarios.
type ModelSelector struct {
	atomic *config.AtomicConfig
}

// NewModelSelector creates a new model selector.
func NewModelSelector(atomic *config.AtomicConfig) *ModelSelector {
	return &ModelSelector{atomic: atomic}
}

// Select determines which model to use for a request.
func (s *ModelSelector) Select(messages []types.Message, tokenCount int, requestedModel string, isStreaming bool) (*Result, error) {
	cfg := s.atomic.Get()

	// Honour the model picked in Claude Code, unless it is a Claude model (those
	// are not served by OpenCode, so scenario routing picks a replacement).
	if cfg.RespectRequestedModel && requestedModel != "" && !IsClaudeModel(requestedModel) {
		// Keep the request's own temperature/max_tokens: the per-scenario caps
		// would otherwise truncate long agentic outputs.
		primary := config.ModelConfig{
			Provider: "opencode-go",
			ModelID:  requestedModel,
		}
		return &Result{
			Primary:   primary,
			Fallbacks: cfg.Fallbacks["default"],
			Scenario:  ScenarioDefault,
		}, nil
	}

	// Detect scenario
	scenario := detectScenario(messages, tokenCount, cfg, isStreaming)

	// Get primary model for scenario
	primary, ok := cfg.Models[string(scenario)]
	if !ok {
		// Fall back to default
		primary, ok = cfg.Models["default"]
		if !ok {
			// Create a minimal default
			primary = config.ModelConfig{
				Provider:    "opencode-go",
				ModelID:     "kimi-k2.6",
				Temperature: 0.7,
				MaxTokens:   4096,
			}
		}
	}

	// Get fallbacks for scenario
	fallbacks := cfg.Fallbacks[string(scenario)]
	if len(fallbacks) == 0 {
		fallbacks = cfg.Fallbacks["default"]
	}

	return &Result{
		Primary:   primary,
		Fallbacks: fallbacks,
		Scenario:  scenario,
	}, nil
}

// detectScenario determines the request scenario based on content analysis.
func detectScenario(messages []types.Message, tokenCount int, cfg *config.Config, isStreaming bool) Scenario {
	// Streaming fast mode (unless explicitly disabled)
	if isStreaming && !cfg.EnableStreamingScenarioRouting {
		return ScenarioFast
	}

	// Long context check
	if tokenCount > 0 {
		if longContextCfg, ok := cfg.Models["long_context"]; ok && longContextCfg.ContextThreshold > 0 {
			if tokenCount > longContextCfg.ContextThreshold {
				return ScenarioLongContext
			}
		} else if tokenCount > 80000 {
			return ScenarioLongContext
		}
	}

	// Keyword scenarios look at what the user last asked, not the whole history.
	lowerText := strings.ToLower(lastUserText(messages))

	if thinkPattern.MatchString(lowerText) {
		return ScenarioThink
	}

	if complexPattern.MatchString(lowerText) {
		return ScenarioComplex
	}

	if backgroundPattern.MatchString(lowerText) {
		return ScenarioBackground
	}

	return ScenarioDefault
}

var (
	thinkPattern      = regexp.MustCompile(`\b(think(ing)?|plan(s|ning)?|reason(ing)?|analy[sz]e)\b`)
	complexPattern    = regexp.MustCompile(`\b(architect(ure)?|refactor(ing)?|complex|design|structure)\b`)
	backgroundPattern = regexp.MustCompile(`\b(read file|list directory|grep|find file|cat)\b`)
)

// lastUserText returns the text of the most recent user message that has any.
// Turns that only carry tool_result blocks are skipped.
func lastUserText(messages []types.Message) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role != "user" {
			continue
		}

		var text string
		for _, block := range messages[i].ContentBlocks() {
			if block.Type == "text" {
				text += block.Text + " "
			}
		}
		if strings.TrimSpace(text) != "" {
			return text
		}
	}
	return ""
}

// IsClaudeModel reports whether a model ID refers to an official Claude model
// (or one of Claude Code's model aliases), as opposed to an OpenCode model.
func IsClaudeModel(modelID string) bool {
	id := strings.ToLower(modelID)
	if strings.HasPrefix(id, "claude-") {
		return true
	}

	switch strings.TrimSuffix(id, "[1m]") {
	case "", "sonnet", "opus", "haiku", "best", "opusplan", "default":
		return true
	}
	return false
}
