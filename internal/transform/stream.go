// Package transform handles real-time SSE stream transformation.
package transform

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/nilparra-dev/opencode-go-cc/pkg/types"
)

// StreamTransformer converts OpenAI SSE streams to Anthropic SSE format.
type StreamTransformer struct {
	originalModel string
	writer        io.Writer
	flusher       interface{ Flush() error }
}

// NewStreamTransformer creates a new stream transformer.
func NewStreamTransformer(originalModel string, w io.Writer) *StreamTransformer {
	t := &StreamTransformer{
		originalModel: originalModel,
		writer:        w,
	}
	// Try to get flusher for HTTP response writer
	if f, ok := w.(interface{ Flush() error }); ok {
		t.flusher = f
	}
	return t
}

// Transform reads OpenAI SSE from reader and writes Anthropic SSE to writer.
func (t *StreamTransformer) Transform(reader io.Reader) error {
	scanner := bufio.NewScanner(reader)

	var messageID string
	var contentIndex int
	var toolCallIndex int
	var hasSentThinking bool
	var hasSentToolCall bool
	var currentToolCall *types.ToolCall
	var toolCallBuffer strings.Builder

	// Send message_start
	startData, _ := json.Marshal(map[string]interface{}{
		"type": "message_start",
		"message": map[string]interface{}{
			"id":    generateMessageID(),
			"type":  "message",
			"role":  "assistant",
			"model": t.originalModel,
			"usage": map[string]int{"input_tokens": 0, "output_tokens": 0},
		},
	})
	if err := t.writeEvent("message_start", startData); err != nil {
		return err
	}

	for scanner.Scan() {
		line := scanner.Text()

		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}

		var chunk types.StreamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue // skip malformed chunks
		}

		if messageID == "" {
			messageID = chunk.ID
		}

		if len(chunk.Choices) == 0 {
			if chunk.Usage != nil {
				// Final usage chunk
				usageData, _ := json.Marshal(map[string]interface{}{
					"type": "message_delta",
					"delta": map[string]interface{}{},
					"usage": map[string]int{
						"output_tokens": chunk.Usage.CompletionTokens,
					},
				})
				_ = t.writeEvent("message_delta", usageData)
			}
			continue
		}

		delta := chunk.Choices[0].Delta

		// Handle reasoning content (thinking)
		if delta.ReasoningContent != nil && *delta.ReasoningContent != "" {
			if !hasSentThinking {
				tcData, _ := json.Marshal(map[string]interface{}{
					"type":  "content_block_start",
					"index": contentIndex,
					"content_block": map[string]interface{}{
						"type":     "thinking",
						"thinking": "",
					},
				})
				if err := t.writeEvent("content_block_start", tcData); err != nil {
					return err
				}
				hasSentThinking = true
			}

			tdData, _ := json.Marshal(map[string]interface{}{
				"type":  "content_block_delta",
				"index": contentIndex,
				"delta": map[string]interface{}{
					"type":          "thinking_delta",
					"thinking":      *delta.ReasoningContent,
				},
			})
			if err := t.writeEvent("content_block_delta", tdData); err != nil {
				return err
			}
		}

		// Handle tool calls
		for _, tc := range delta.ToolCalls {
			if tc.ID != "" && (currentToolCall == nil || currentToolCall.ID != tc.ID) {
				// Finish previous tool call if any
				if currentToolCall != nil {
					t.finishToolCall(contentIndex, currentToolCall, toolCallBuffer.String())
					contentIndex++
				}

				currentToolCall = &tc
				toolCallBuffer.Reset()
				toolCallIndex = contentIndex

				// Send content_block_start for tool_use
				name := tc.Function.Name
				if name == "" {
					name = "unknown"
				}
				tcData, _ := json.Marshal(map[string]interface{}{
					"type":  "content_block_start",
					"index": contentIndex,
					"content_block": map[string]interface{}{
						"type": "tool_use",
						"id":   tc.ID,
						"name": name,
						"input": map[string]interface{}{},
					},
				})
				if err := t.writeEvent("content_block_start", tcData); err != nil {
					return err
				}
				hasSentToolCall = true
			}

			if tc.Function.Arguments != "" {
				toolCallBuffer.WriteString(tc.Function.Arguments)

				partialJSON := tc.Function.Arguments
				tdData, _ := json.Marshal(map[string]interface{}{
					"type":  "content_block_delta",
					"index": toolCallIndex,
					"delta": map[string]interface{}{
						"type":         "input_json_delta",
						"partial_json": partialJSON,
					},
				})
				if err := t.writeEvent("content_block_delta", tdData); err != nil {
					return err
				}
			}
		}

		// Handle text content
		if delta.Content != "" {
			if !hasSentThinking && !hasSentToolCall {
				// First text content block
				tcData, _ := json.Marshal(map[string]interface{}{
					"type":  "content_block_start",
					"index": contentIndex,
					"content_block": map[string]interface{}{
						"type": "text",
						"text": "",
					},
				})
				if err := t.writeEvent("content_block_start", tcData); err != nil {
					return err
				}
			}

			tdData, _ := json.Marshal(map[string]interface{}{
				"type":  "content_block_delta",
				"index": contentIndex,
				"delta": map[string]interface{}{
					"type": "text_delta",
					"text": delta.Content,
				},
			})
			if err := t.writeEvent("content_block_delta", tdData); err != nil {
				return err
			}
		}
	}

	// Finish any remaining tool call
	if currentToolCall != nil {
		t.finishToolCall(toolCallIndex, currentToolCall, toolCallBuffer.String())
	}

	// Send content_block_stop for the last block
	stopData, _ := json.Marshal(map[string]interface{}{
		"type":  "content_block_stop",
		"index": contentIndex,
	})
	if err := t.writeEvent("content_block_stop", stopData); err != nil {
		return err
	}

	// Send message_stop
	msgStopData, _ := json.Marshal(map[string]interface{}{
		"type": "message_stop",
	})
	if err := t.writeEvent("message_stop", msgStopData); err != nil {
		return err
	}

	return scanner.Err()
}

func (t *StreamTransformer) finishToolCall(index int, tc *types.ToolCall, arguments string) {
	// Send content_block_stop for tool_use
	stopData, _ := json.Marshal(map[string]interface{}{
		"type":  "content_block_stop",
		"index": index,
	})
	_ = t.writeEvent("content_block_stop", stopData)
}

func (t *StreamTransformer) writeEvent(event string, data []byte) error {
	_, err := fmt.Fprintf(t.writer, "event: %s\ndata: %s\n\n", event, string(data))
	if err != nil {
		return err
	}
	if t.flusher != nil {
		return t.flusher.Flush()
	}
	return nil
}

var messageIDCounter int

func generateMessageID() string {
	messageIDCounter++
	return fmt.Sprintf("msg_%010d", messageIDCounter)
}
