// Package router handles model selection based on request scenarios.
package router

import (
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

	// Check if we should respect the requested model
	if cfg.RespectRequestedModel && requestedModel != "" {
		primary := config.ModelConfig{
			Provider: "opencode-go",
			ModelID:  requestedModel,
		}
		if def, ok := cfg.Models["default"]; ok {
			primary.Temperature = def.Temperature
			primary.MaxTokens = def.MaxTokens
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

	// Analyze system prompt and user messages for keywords
	var allText string
	for _, msg := range messages {
		if msg.Role == "system" || msg.Role == "user" {
			blocks := msg.ContentBlocks()
			for _, block := range blocks {
				allText += block.Text + " "
			}
		}
	}

	lowerText := strings.ToLower(allText)

	// Think scenario
	if containsAny(lowerText, "think", "plan", "reason", "analyze") {
		return ScenarioThink
	}

	// Complex scenario
	if containsAny(lowerText, "architect", "refactor", "complex", "design", "structure") {
		return ScenarioComplex
	}

	// Background scenario (simple read operations)
	if containsAny(lowerText, "read file", "list directory", "grep", "find file", "cat ") {
		return ScenarioBackground
	}

	return ScenarioDefault
}

// containsAny checks if text contains any of the keywords.
func containsAny(text string, keywords ...string) bool {
	for _, kw := range keywords {
		if strings.Contains(text, kw) {
			return true
		}
	}
	return false
}
