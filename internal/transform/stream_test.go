package transform

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

type sseEvent struct {
	Name string
	Data map[string]interface{}
}

func parseSSE(t *testing.T, raw string) []sseEvent {
	t.Helper()

	var events []sseEvent
	for _, frame := range strings.Split(strings.TrimSpace(raw), "\n\n") {
		var ev sseEvent
		for _, line := range strings.Split(frame, "\n") {
			switch {
			case strings.HasPrefix(line, "event: "):
				ev.Name = strings.TrimPrefix(line, "event: ")
			case strings.HasPrefix(line, "data: "):
				if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &ev.Data); err != nil {
					t.Fatalf("invalid event data %q: %v", line, err)
				}
			}
		}
		events = append(events, ev)
	}
	return events
}

func runTransform(t *testing.T, upstream string) []sseEvent {
	t.Helper()

	var out bytes.Buffer
	if err := NewStreamTransformer("claude-test", &out).Transform(strings.NewReader(upstream)); err != nil {
		t.Fatalf("Transform returned error: %v", err)
	}
	return parseSSE(t, out.String())
}

// checkWellFormed asserts the structural rules Claude Code relies on: blocks are
// opened and closed in order, one at a time, deltas target the open block, and
// the stream ends with message_delta + message_stop.
func checkWellFormed(t *testing.T, events []sseEvent) {
	t.Helper()

	if events[0].Name != "message_start" {
		t.Fatalf("first event = %q, want message_start", events[0].Name)
	}
	if n := len(events); events[n-2].Name != "message_delta" || events[n-1].Name != "message_stop" {
		t.Fatalf("stream must end with message_delta, message_stop; got %q, %q", events[n-2].Name, events[n-1].Name)
	}

	open := -1
	next := 0
	for _, ev := range events {
		index := -1
		if v, ok := ev.Data["index"].(float64); ok {
			index = int(v)
		}

		switch ev.Name {
		case "content_block_start":
			if open != -1 {
				t.Fatalf("block %d started while block %d is still open", index, open)
			}
			if index != next {
				t.Fatalf("block index = %d, want %d", index, next)
			}
			open = index
			next++
		case "content_block_delta":
			if index != open {
				t.Fatalf("delta for block %d but open block is %d", index, open)
			}
		case "content_block_stop":
			if index != open {
				t.Fatalf("stop for block %d but open block is %d", index, open)
			}
			open = -1
		}
	}
	if open != -1 {
		t.Fatalf("block %d never closed", open)
	}
}

func blockTypes(events []sseEvent) []string {
	var types []string
	for _, ev := range events {
		if ev.Name == "content_block_start" {
			block := ev.Data["content_block"].(map[string]interface{})
			types = append(types, block["type"].(string))
		}
	}
	return types
}

func stopReason(events []sseEvent) string {
	delta := events[len(events)-2].Data["delta"].(map[string]interface{})
	return delta["stop_reason"].(string)
}

func TestStreamTextOnly(t *testing.T) {
	events := runTransform(t, `data: {"id":"1","choices":[{"delta":{"content":"Hel"}}]}

data: {"id":"1","choices":[{"delta":{"content":"lo"},"finish_reason":"stop"}]}

data: {"id":"1","choices":[],"usage":{"prompt_tokens":12,"completion_tokens":3}}

data: [DONE]

`)

	checkWellFormed(t, events)

	if got := strings.Join(blockTypes(events), ","); got != "text" {
		t.Fatalf("blocks = %s, want text", got)
	}
	if got := stopReason(events); got != "end_turn" {
		t.Fatalf("stop_reason = %s, want end_turn", got)
	}

	usage := events[len(events)-2].Data["usage"].(map[string]interface{})
	if usage["input_tokens"].(float64) != 12 || usage["output_tokens"].(float64) != 3 {
		t.Fatalf("unexpected usage: %v", usage)
	}
}

func TestStreamThinkingThenTextThenToolClosesEachBlock(t *testing.T) {
	events := runTransform(t, `data: {"choices":[{"delta":{"reasoning_content":"hmm"}}]}

data: {"choices":[{"delta":{"content":"I will read it."}}]}

data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"Read","arguments":""}}]}}]}

data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"path\":"}}]}}]}

data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"\"a.go\"}"}}]},"finish_reason":"tool_calls"}]}

data: [DONE]

`)

	checkWellFormed(t, events)

	if got := strings.Join(blockTypes(events), ","); got != "thinking,text,tool_use" {
		t.Fatalf("blocks = %s, want thinking,text,tool_use", got)
	}
	if got := stopReason(events); got != "tool_use" {
		t.Fatalf("stop_reason = %s, want tool_use", got)
	}

	// The tool arguments must add up to the original JSON.
	var args string
	for _, ev := range events {
		if ev.Name != "content_block_delta" {
			continue
		}
		delta := ev.Data["delta"].(map[string]interface{})
		if delta["type"] == "input_json_delta" {
			args += delta["partial_json"].(string)
		}
	}
	if args != `{"path":"a.go"}` {
		t.Fatalf("tool arguments = %q", args)
	}
}

func TestStreamParallelToolCalls(t *testing.T) {
	events := runTransform(t, `data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_a","function":{"name":"Read","arguments":"{\"p\":1}"}}]}}]}

data: {"choices":[{"delta":{"tool_calls":[{"index":1,"id":"call_b","function":{"name":"Grep","arguments":"{\"q\":2}"}}]},"finish_reason":"tool_calls"}]}

data: [DONE]

`)

	checkWellFormed(t, events)

	if got := strings.Join(blockTypes(events), ","); got != "tool_use,tool_use" {
		t.Fatalf("blocks = %s, want two tool_use blocks", got)
	}
}

func TestStreamParallelToolCallsRepeatingIDInEveryChunk(t *testing.T) {
	// Some providers repeat the call id on every fragment of the same call.
	events := runTransform(t, `data: {"choices":[{"delta":{"tool_calls":[{"id":"call_a","function":{"name":"Read","arguments":"{\"p\":"}}]}}]}

data: {"choices":[{"delta":{"tool_calls":[{"id":"call_a","function":{"arguments":"1}"}}]}}]}

data: {"choices":[{"delta":{"tool_calls":[{"id":"call_b","function":{"name":"Grep","arguments":"{}"}}]}}]}

data: [DONE]

`)

	checkWellFormed(t, events)

	if got := strings.Join(blockTypes(events), ","); got != "tool_use,tool_use" {
		t.Fatalf("blocks = %s, want two tool_use blocks", got)
	}
}

func TestStreamToolCallWithoutArgumentsGetsEmptyObject(t *testing.T) {
	events := runTransform(t, `data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_1","function":{"name":"Status"}}]},"finish_reason":"tool_calls"}]}

data: [DONE]

`)

	checkWellFormed(t, events)

	found := false
	for _, ev := range events {
		if ev.Name == "content_block_delta" {
			delta := ev.Data["delta"].(map[string]interface{})
			found = found || delta["partial_json"] == "{}"
		}
	}
	if !found {
		t.Fatal("expected an input_json_delta of {} for a tool call with no arguments")
	}
}

func TestStreamFinishReasonStopWithToolCallsBecomesToolUse(t *testing.T) {
	events := runTransform(t, `data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"c","function":{"name":"X","arguments":"{}"}}]},"finish_reason":"stop"}]}

data: [DONE]

`)

	if got := stopReason(events); got != "tool_use" {
		t.Fatalf("stop_reason = %s, want tool_use", got)
	}
}

func TestStreamLengthMapsToMaxTokens(t *testing.T) {
	events := runTransform(t, `data: {"choices":[{"delta":{"content":"cut"},"finish_reason":"length"}]}

data: [DONE]

`)

	if got := stopReason(events); got != "max_tokens" {
		t.Fatalf("stop_reason = %s, want max_tokens", got)
	}
}

func TestStreamEmptyResponseStillEmitsABlock(t *testing.T) {
	events := runTransform(t, "data: [DONE]\n\n")

	checkWellFormed(t, events)
	if got := strings.Join(blockTypes(events), ","); got != "text" {
		t.Fatalf("blocks = %s, want one empty text block", got)
	}
}

func TestStreamHandlesLinesLargerThanScannerLimit(t *testing.T) {
	big := strings.Repeat("x", 200*1024)
	events := runTransform(t, `data: {"choices":[{"delta":{"content":"`+big+`"},"finish_reason":"stop"}]}

data: [DONE]

`)

	checkWellFormed(t, events)

	for _, ev := range events {
		if ev.Name == "content_block_delta" {
			text := ev.Data["delta"].(map[string]interface{})["text"].(string)
			if len(text) != len(big) {
				t.Fatalf("delta text length = %d, want %d", len(text), len(big))
			}
			return
		}
	}
	t.Fatal("no text delta emitted")
}

type flushRecorder struct {
	bytes.Buffer
	flushes int
}

func (f *flushRecorder) Flush() { f.flushes++ }

func TestStreamFlushesAfterEveryEvent(t *testing.T) {
	rec := &flushRecorder{}
	err := NewStreamTransformer("m", rec).Transform(strings.NewReader(`data: {"choices":[{"delta":{"content":"a"}}]}

data: [DONE]

`))
	if err != nil {
		t.Fatal(err)
	}

	events := len(parseSSE(t, rec.String()))
	if rec.flushes != events {
		t.Fatalf("flushes = %d, want one per event (%d)", rec.flushes, events)
	}
}
