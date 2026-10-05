package router

import (
	"encoding/json"
	"testing"

	"github.com/nilparra-dev/opencode-go-cc/internal/config"
	"github.com/nilparra-dev/opencode-go-cc/pkg/types"
)

func userText(text string) types.Message {
	content, _ := json.Marshal(text)
	return types.Message{Role: "user", Content: content}
}

func toolResultOnly() types.Message {
	return types.Message{Role: "user", Content: json.RawMessage(`[{"type":"tool_result","tool_use_id":"x","content":"please plan the architecture"}]`)}
}

func TestIsClaudeModel(t *testing.T) {
	for _, id := range []string{"claude-sonnet-4-5", "Claude-Opus-5-5", "claude-haiku-4-5-20251001", "sonnet", "opus[1m]", "haiku", "opusplan", ""} {
		if !IsClaudeModel(id) {
			t.Errorf("IsClaudeModel(%q) = false, want true", id)
		}
	}
	for _, id := range []string{"kimi-k2.6", "glm-5.1", "qwen3.7-max", "minimax-m2.5", "deepseek-v4-pro"} {
		if IsClaudeModel(id) {
			t.Errorf("IsClaudeModel(%q) = true, want false", id)
		}
	}
}

func TestSelectRespectsRequestedOpenCodeModelWithoutCappingTokens(t *testing.T) {
	cfg := config.DefaultConfig()
	sel := NewModelSelector(config.NewAtomicConfig(cfg, ""))

	res, err := sel.Select([]types.Message{userText("hi")}, 0, "glm-5.1", true)
	if err != nil {
		t.Fatal(err)
	}

	if res.Primary.ModelID != "glm-5.1" {
		t.Fatalf("primary = %q, want glm-5.1", res.Primary.ModelID)
	}
	if res.Primary.MaxTokens != 0 || res.Primary.Temperature != 0 {
		t.Fatalf("requested model must keep the request's own limits, got max_tokens=%d temperature=%v",
			res.Primary.MaxTokens, res.Primary.Temperature)
	}
}

func TestSelectDoesNotSendClaudeModelsToOpenCode(t *testing.T) {
	cfg := config.DefaultConfig()
	sel := NewModelSelector(config.NewAtomicConfig(cfg, ""))

	res, err := sel.Select([]types.Message{userText("hi")}, 0, "claude-sonnet-4-5", true)
	if err != nil {
		t.Fatal(err)
	}
	if IsClaudeModel(res.Primary.ModelID) {
		t.Fatalf("a Claude model ID was selected for OpenCode: %q", res.Primary.ModelID)
	}
}

func TestStreamingRequestsUseScenarioRoutingByDefault(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.RespectRequestedModel = false
	sel := NewModelSelector(config.NewAtomicConfig(cfg, ""))

	res, err := sel.Select([]types.Message{userText("please plan the migration")}, 0, "", true)
	if err != nil {
		t.Fatal(err)
	}
	if res.Scenario != ScenarioThink {
		t.Fatalf("scenario = %q, want %q", res.Scenario, ScenarioThink)
	}
}

func TestScenarioKeywordsMatchWholeWordsOnly(t *testing.T) {
	cfg := config.DefaultConfig()

	tests := []struct {
		text string
		want Scenario
	}{
		{"can you explain this function", ScenarioDefault}, // "plan" inside "explain"
		{"the planet is round", ScenarioDefault},
		{"this is a designed API", ScenarioDefault},
		{"please plan the work", ScenarioThink},
		{"Refactor the parser", ScenarioComplex},
		{"grep for TODO", ScenarioBackground},
	}

	for _, tt := range tests {
		got := detectScenario([]types.Message{userText(tt.text)}, 0, cfg, true)
		if got != tt.want {
			t.Errorf("detectScenario(%q) = %q, want %q", tt.text, got, tt.want)
		}
	}
}

func TestScenarioUsesLastUserRequestNotHistory(t *testing.T) {
	cfg := config.DefaultConfig()

	messages := []types.Message{
		userText("please plan the migration"),
		{Role: "assistant", Content: json.RawMessage(`"ok"`)},
		userText("now rename the variable"),
		toolResultOnly(), // tool output mentioning "plan" must not count
	}

	if got := detectScenario(messages, 0, cfg, true); got != ScenarioDefault {
		t.Fatalf("scenario = %q, want %q", got, ScenarioDefault)
	}
}
