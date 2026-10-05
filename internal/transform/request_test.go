package transform

import (
	"encoding/json"
	"testing"

	"github.com/nilparra-dev/opencode-go-cc/internal/config"
	"github.com/nilparra-dev/opencode-go-cc/pkg/types"
)

func TestToolResultContentReachesTheModel(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{"string content", `"file contents"`, "file contents"},
		{"block array", `[{"type":"text","text":"line 1\n"},{"type":"text","text":"line 2"}]`, "line 1\nline 2"},
		{"image block", `[{"type":"text","text":"shot: "},{"type":"image","source":{}}]`, "shot: [Image]"},
		{"missing content", ``, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			block := `{"type":"tool_result","tool_use_id":"call_1"`
			if tt.content != "" {
				block += `,"content":` + tt.content
			}
			block += `}`

			req := &types.MessageRequest{
				Model: "kimi-k2.6",
				Messages: []types.Message{
					{Role: "user", Content: json.RawMessage(`[` + block + `]`)},
				},
			}

			out, err := NewRequestTransformer().TransformRequest(req, config.ModelConfig{ModelID: "kimi-k2.6"})
			if err != nil {
				t.Fatal(err)
			}

			if len(out.Messages) != 1 {
				t.Fatalf("got %d messages, want 1", len(out.Messages))
			}
			msg := out.Messages[0]
			if msg.Role != "tool" || msg.ToolCallID != "call_1" || msg.Content != tt.want {
				t.Fatalf("got role=%q id=%q content=%q, want tool/call_1/%q", msg.Role, msg.ToolCallID, msg.Content, tt.want)
			}
		})
	}
}

func TestNonStreamingToolArgumentsThatAreNotJSONDoNotBreakTheResponse(t *testing.T) {
	resp := &types.ChatCompletionResponse{
		Choices: []types.Choice{{
			Message: types.ChatMessage{
				Content: "ok",
				ToolCalls: []types.ToolCall{
					{ID: "c1", Function: types.FunctionCall{Name: "Read", Arguments: `{"truncated":`}},
				},
			},
			FinishReason: "tool_calls",
		}},
	}

	out, err := NewResponseTransformer().TransformResponse(resp, "claude-test")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := json.Marshal(out); err != nil {
		t.Fatalf("response is not serializable: %v", err)
	}
	if out.Content[0].Type != "text" || out.Content[1].Type != "tool_use" {
		t.Fatalf("expected text before tool_use, got %s, %s", out.Content[0].Type, out.Content[1].Type)
	}
}
