// Package transform handles real-time SSE stream transformation.
package transform

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync/atomic"

	"github.com/nilparra-dev/opencode-go-cc/pkg/types"
)

type blockKind int

const (
	blockNone blockKind = iota
	blockThinking
	blockText
	blockTool
)

// StreamTransformer converts OpenAI SSE streams to Anthropic SSE format.
//
// Anthropic streams are a strict sequence of content blocks (one open at a time),
// so the transformer tracks the open block and closes it whenever the kind of
// content changes (thinking -> text -> tool_use ...).
type StreamTransformer struct {
	originalModel string
	writer        io.Writer
	flush         func()

	nextIndex int
	open      blockKind
	openIndex int

	curToolID    string
	curToolIndex int
	toolArgsSeen bool
	toolSeq      int
	sawTool      bool
	sawBlock     bool

	stopReason   string
	inputTokens  int
	outputTokens int
}

// NewStreamTransformer creates a new stream transformer.
func NewStreamTransformer(originalModel string, w io.Writer) *StreamTransformer {
	t := &StreamTransformer{
		originalModel: originalModel,
		writer:        w,
		curToolIndex:  -1,
	}

	switch f := w.(type) {
	case interface{ Flush() }:
		t.flush = f.Flush
	case interface{ Flush() error }:
		t.flush = func() { _ = f.Flush() }
	}

	return t
}

// Transform reads OpenAI SSE from reader and writes Anthropic SSE to writer.
// It returns an error if the upstream stream breaks or the client goes away.
func (t *StreamTransformer) Transform(reader io.Reader) error {
	if err := t.writeJSONEvent("message_start", map[string]interface{}{
		"type": "message_start",
		"message": map[string]interface{}{
			"id":            generateMessageID(),
			"type":          "message",
			"role":          "assistant",
			"model":         t.originalModel,
			"content":       []interface{}{},
			"stop_reason":   nil,
			"stop_sequence": nil,
			"usage":         map[string]int{"input_tokens": 0, "output_tokens": 0},
		},
	}); err != nil {
		return err
	}

	// bufio.Reader (unlike bufio.Scanner) has no per-line size limit.
	br := bufio.NewReaderSize(reader, 64*1024)
	for {
		line, readErr := br.ReadString('\n')
		if line != "" {
			done, err := t.handleLine(line)
			if err != nil {
				return err
			}
			if done {
				break
			}
		}
		if readErr != nil {
			if readErr == io.EOF {
				break
			}
			return readErr
		}
	}

	return t.finish()
}

// handleLine processes one raw SSE line. It reports true once [DONE] is seen.
func (t *StreamTransformer) handleLine(line string) (bool, error) {
	line = strings.TrimRight(line, "\r\n")

	data, ok := strings.CutPrefix(line, "data:")
	if !ok {
		return false, nil
	}
	data = strings.TrimPrefix(data, " ")

	if data == "[DONE]" {
		return true, nil
	}

	var chunk types.StreamChunk
	if err := json.Unmarshal([]byte(data), &chunk); err != nil {
		return false, nil // skip malformed chunks
	}

	if chunk.Usage != nil {
		t.inputTokens = chunk.Usage.PromptTokens
		t.outputTokens = chunk.Usage.CompletionTokens
	}

	if len(chunk.Choices) == 0 {
		return false, nil
	}

	choice := chunk.Choices[0]
	delta := choice.Delta

	reasoning := delta.ReasoningContent
	if reasoning == nil {
		reasoning = delta.Reasoning
	}
	if reasoning != nil && *reasoning != "" {
		if err := t.emitDelta(blockThinking, "thinking_delta", "thinking", *reasoning); err != nil {
			return false, err
		}
	}

	if delta.Content != "" {
		if err := t.emitDelta(blockText, "text_delta", "text", delta.Content); err != nil {
			return false, err
		}
	}

	for _, tc := range delta.ToolCalls {
		if err := t.handleToolCall(tc); err != nil {
			return false, err
		}
	}

	if choice.FinishReason != "" {
		t.stopReason = stopReasonFor(choice.FinishReason)
	}

	return false, nil
}

// emitDelta opens a block of the given kind if needed and writes a delta to it.
func (t *StreamTransformer) emitDelta(kind blockKind, deltaType, field, value string) error {
	if t.open != kind {
		if err := t.closeBlock(); err != nil {
			return err
		}

		blockType := "text"
		if kind == blockThinking {
			blockType = "thinking"
		}
		if err := t.openBlock(kind, map[string]interface{}{
			"type": blockType,
			field:  "",
		}); err != nil {
			return err
		}
	}

	return t.writeJSONEvent("content_block_delta", map[string]interface{}{
		"type":  "content_block_delta",
		"index": t.openIndex,
		"delta": map[string]interface{}{
			"type": deltaType,
			field:  value,
		},
	})
}

// handleToolCall maps one streamed OpenAI tool_call fragment onto a tool_use block.
func (t *StreamTransformer) handleToolCall(tc types.ToolCall) error {
	isNew := t.open != blockTool ||
		(tc.ID != "" && tc.ID != t.curToolID) ||
		(tc.Index != nil && *tc.Index != t.curToolIndex)

	if isNew {
		if err := t.closeBlock(); err != nil {
			return err
		}

		id := tc.ID
		if id == "" {
			t.toolSeq++
			id = fmt.Sprintf("toolu_%s_%d", generateMessageID(), t.toolSeq)
		}
		name := tc.Function.Name
		if name == "" {
			name = "unknown"
		}

		if err := t.openBlock(blockTool, map[string]interface{}{
			"type":  "tool_use",
			"id":    id,
			"name":  name,
			"input": map[string]interface{}{},
		}); err != nil {
			return err
		}

		t.curToolID = id
		t.curToolIndex = -1
		if tc.Index != nil {
			t.curToolIndex = *tc.Index
		}
		t.toolArgsSeen = false
		t.sawTool = true
	}

	if tc.Function.Arguments != "" {
		t.toolArgsSeen = true
		return t.writeJSONEvent("content_block_delta", map[string]interface{}{
			"type":  "content_block_delta",
			"index": t.openIndex,
			"delta": map[string]interface{}{
				"type":         "input_json_delta",
				"partial_json": tc.Function.Arguments,
			},
		})
	}

	return nil
}

func (t *StreamTransformer) openBlock(kind blockKind, block map[string]interface{}) error {
	t.open = kind
	t.openIndex = t.nextIndex
	t.nextIndex++
	t.sawBlock = true

	return t.writeJSONEvent("content_block_start", map[string]interface{}{
		"type":          "content_block_start",
		"index":         t.openIndex,
		"content_block": block,
	})
}

// closeBlock ends the currently open block, if any.
func (t *StreamTransformer) closeBlock() error {
	if t.open == blockNone {
		return nil
	}

	// A tool_use block with no streamed arguments still needs a valid JSON input.
	if t.open == blockTool && !t.toolArgsSeen {
		if err := t.writeJSONEvent("content_block_delta", map[string]interface{}{
			"type":  "content_block_delta",
			"index": t.openIndex,
			"delta": map[string]interface{}{
				"type":         "input_json_delta",
				"partial_json": "{}",
			},
		}); err != nil {
			return err
		}
	}

	index := t.openIndex
	t.open = blockNone
	t.curToolID = ""
	t.curToolIndex = -1

	return t.writeJSONEvent("content_block_stop", map[string]interface{}{
		"type":  "content_block_stop",
		"index": index,
	})
}

// finish closes the open block and emits message_delta + message_stop.
func (t *StreamTransformer) finish() error {
	if err := t.closeBlock(); err != nil {
		return err
	}

	if !t.sawBlock {
		if err := t.openBlock(blockText, map[string]interface{}{"type": "text", "text": ""}); err != nil {
			return err
		}
		if err := t.closeBlock(); err != nil {
			return err
		}
	}

	stopReason := t.stopReason
	if stopReason == "" {
		stopReason = "end_turn"
	}
	// Some providers report finish_reason "stop" even when they emitted tool calls.
	if stopReason == "end_turn" && t.sawTool {
		stopReason = "tool_use"
	}

	if err := t.writeJSONEvent("message_delta", map[string]interface{}{
		"type": "message_delta",
		"delta": map[string]interface{}{
			"stop_reason":   stopReason,
			"stop_sequence": nil,
		},
		"usage": map[string]int{
			"input_tokens":  t.inputTokens,
			"output_tokens": t.outputTokens,
		},
	}); err != nil {
		return err
	}

	return t.writeJSONEvent("message_stop", map[string]interface{}{"type": "message_stop"})
}

func (t *StreamTransformer) writeJSONEvent(event string, payload map[string]interface{}) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	if _, err := fmt.Fprintf(t.writer, "event: %s\ndata: %s\n\n", event, data); err != nil {
		return err
	}
	if t.flush != nil {
		t.flush()
	}
	return nil
}

// stopReasonFor maps an OpenAI finish_reason to an Anthropic stop_reason.
func stopReasonFor(reason string) string {
	return (&ResponseTransformer{}).mapFinishReason(reason)
}

var messageIDCounter atomic.Uint64

func generateMessageID() string {
	return fmt.Sprintf("msg_%010d", messageIDCounter.Add(1))
}
