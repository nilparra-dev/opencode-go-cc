// Package transform handles request and response format conversion.
package transform

import (
	"encoding/json"
	"fmt"

	"github.com/nilparra-dev/opencode-go-cc/pkg/types"
)

// ResponseTransformer converts OpenAI responses to Anthropic format.
type ResponseTransformer struct{}

// NewResponseTransformer creates a new response transformer.
func NewResponseTransformer() *ResponseTransformer {
	return &ResponseTransformer{}
}

// nonNegative clamps an integer to zero.
func nonNegative(n int) int {
	if n < 0 {
		return 0
	}
	return n
}

// TransformResponse converts an OpenAI ChatCompletionResponse to Anthropic MessageResponse.
func (t *ResponseTransformer) TransformResponse(
	openaiResp *types.ChatCompletionResponse,
	originalModel string,
) (*types.MessageResponse, error) {
	if len(openaiResp.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}

	choice := openaiResp.Choices[0]

	contentBlocks, err := t.transformContent(choice.Message)
	if err != nil {
		return nil, fmt.Errorf("failed to transform content: %w", err)
	}

	stopReason := t.mapFinishReason(choice.FinishReason)

	anthropicResp := &types.MessageResponse{
		ID:         openaiResp.ID,
		Type:       "message",
		Role:       "assistant",
		Content:    contentBlocks,
		Model:      originalModel,
		StopReason: stopReason,
		Usage: types.Usage{
			InputTokens:              nonNegative(openaiResp.Usage.PromptTokens - openaiResp.Usage.PromptCacheHitTokens - openaiResp.Usage.PromptCacheMissTokens),
			OutputTokens:             openaiResp.Usage.CompletionTokens,
			CacheCreationInputTokens: openaiResp.Usage.PromptCacheMissTokens,
			CacheReadInputTokens:     openaiResp.Usage.PromptCacheHitTokens,
		},
	}

	return anthropicResp, nil
}

// transformContent converts an OpenAI message to Anthropic content blocks.
func (t *ResponseTransformer) transformContent(msg types.ChatMessage) ([]types.ContentBlock, error) {
	var blocks []types.ContentBlock

	if msg.ReasoningContent != nil && *msg.ReasoningContent != "" {
		blocks = append(blocks, types.ContentBlock{
			Type:     "thinking",
			Thinking: *msg.ReasoningContent,
		})
	}

	for _, tc := range msg.ToolCalls {
		inputJSON := json.RawMessage(`{}`)
		if tc.Function.Arguments != "" {
			inputJSON = json.RawMessage(tc.Function.Arguments)
		}

		blocks = append(blocks, types.ContentBlock{
			Type:  "tool_use",
			ID:    tc.ID,
			Name:  tc.Function.Name,
			Input: inputJSON,
		})
	}

	if msg.Content != "" {
		blocks = append(blocks, types.ContentBlock{
			Type: "text",
			Text: msg.Content,
		})
	}

	if len(blocks) == 0 {
		blocks = append(blocks, types.ContentBlock{
			Type: "text",
			Text: "",
		})
	}

	return blocks, nil
}

// mapFinishReason maps OpenAI finish reasons to Anthropic stop reasons.
func (t *ResponseTransformer) mapFinishReason(reason string) string {
	switch reason {
	case "stop":
		return "end_turn"
	case "length":
		return "max_tokens"
	case "tool_calls", "tool_use":
		return "tool_use"
	case "content_filter":
		return "end_turn"
	default:
		return "end_turn"
	}
}

// TransformErrorResponse converts an HTTP error into an Anthropic-style error map.
func TransformErrorResponse(statusCode int, message string) map[string]interface{} {
	return map[string]interface{}{
		"type": "error",
		"error": map[string]interface{}{
			"type":    mapHTTPStatusToErrorType(statusCode),
			"message": message,
		},
	}
}

// mapHTTPStatusToErrorType maps HTTP status codes to Anthropic error type strings.
func mapHTTPStatusToErrorType(statusCode int) string {
	switch {
	case statusCode == 400:
		return "invalid_request_error"
	case statusCode == 401:
		return "authentication_error"
	case statusCode == 403:
		return "permission_error"
	case statusCode == 404:
		return "not_found_error"
	case statusCode == 429:
		return "rate_limit_error"
	case statusCode >= 500:
		return "api_error"
	default:
		return "api_error"
	}
}
