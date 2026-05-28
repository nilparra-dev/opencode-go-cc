// Package types provides shared data structures for Anthropic and OpenAI APIs.
package types

import "encoding/json"

// MessageRequest represents an Anthropic Messages API request.
type MessageRequest struct {
	Model       string          `json:"model"`
	Messages    []Message       `json:"messages"`
	System      json.RawMessage `json:"system,omitempty"`
	MaxTokens   int             `json:"max_tokens"`
	Temperature *float64        `json:"temperature,omitempty"`
	TopP        *float64        `json:"top_p,omitempty"`
	Stream      *bool           `json:"stream,omitempty"`
	Tools       []Tool          `json:"tools,omitempty"`
	ToolChoice  json.RawMessage `json:"tool_choice,omitempty"`
	Thinking    json.RawMessage `json:"thinking,omitempty"`
}

// MessageResponse represents an Anthropic Messages API response.
type MessageResponse struct {
	ID           string         `json:"id"`
	Type         string         `json:"type"`
	Role         string         `json:"role"`
	Content      []ContentBlock `json:"content"`
	Model        string         `json:"model"`
	StopReason   string         `json:"stop_reason,omitempty"`
	StopSequence string         `json:"stop_sequence,omitempty"`
	Usage        Usage          `json:"usage"`
}

// Message represents a single message in the conversation.
type Message struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}

// ContentBlock represents a piece of content within a message.
type ContentBlock struct {
	Type     string          `json:"type"`
	Text     string          `json:"text,omitempty"`
	Thinking string          `json:"thinking,omitempty"`
	ID       string          `json:"id,omitempty"`
	Name     string          `json:"name,omitempty"`
	Input    json.RawMessage `json:"input,omitempty"`
	ToolUseID string         `json:"tool_use_id,omitempty"`
}

// SystemContentBlock represents a system prompt block when sent as an array.
type SystemContentBlock struct {
	Type         string          `json:"type"`
	Text         string          `json:"text"`
	CacheControl json.RawMessage `json:"cache_control,omitempty"`
}

// Tool represents an Anthropic tool definition.
type Tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	InputSchema json.RawMessage `json:"input_schema"`
}

// Usage represents token usage in Anthropic API responses.
type Usage struct {
	InputTokens              int `json:"input_tokens"`
	OutputTokens             int `json:"output_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens,omitempty"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens,omitempty"`
}

// SSEEvent represents a server-sent event from Anthropic API.
type SSEEvent struct {
	Event string          `json:"event"`
	Data  json.RawMessage `json:"data"`
}

// SystemText extracts the system prompt as a plain string.
// It handles both string and array formats.
func (r *MessageRequest) SystemText() string {
	if len(r.System) == 0 {
		return ""
	}

	// Try string first
	var s string
	if err := json.Unmarshal(r.System, &s); err == nil {
		return s
	}

	// Try array of blocks
	var blocks []SystemContentBlock
	if err := json.Unmarshal(r.System, &blocks); err == nil {
		var text string
		for _, b := range blocks {
			if b.Type == "text" {
				text += b.Text
			}
		}
		return text
	}

	return ""
}

// ContentBlocks extracts content blocks from a Message.
// It handles both string and array formats.
func (m Message) ContentBlocks() []ContentBlock {
	if len(m.Content) == 0 {
		return nil
	}

	// Try array of blocks first
	var blocks []ContentBlock
	if err := json.Unmarshal(m.Content, &blocks); err == nil {
		return blocks
	}

	// Fallback to plain string
	var s string
	if err := json.Unmarshal(m.Content, &s); err == nil {
		return []ContentBlock{{Type: "text", Text: s}}
	}

	return nil
}

// TextContent returns the concatenated text from content blocks.
func (c ContentBlock) TextContent() string {
	if c.Type == "text" {
		return c.Text
	}
	return ""
}

// GetToolID returns the tool use ID for tool_result blocks.
func (c ContentBlock) GetToolID() string {
	if c.Type == "tool_result" {
		return c.ToolUseID
	}
	return ""
}
