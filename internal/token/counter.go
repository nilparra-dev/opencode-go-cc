// Package token provides token counting utilities.
package token

import (
	"github.com/nilparra-dev/opencode-go-cc/pkg/types"
	"github.com/pkoukk/tiktoken-go"
)

// Counter provides token counting using tiktoken.
type Counter struct {
	encoding *tiktoken.Tiktoken
}

// NewCounter creates a new token counter using cl100k_base.
func NewCounter() (*Counter, error) {
	encoding, err := tiktoken.GetEncoding("cl100k_base")
	if err != nil {
		return nil, err
	}
	return &Counter{encoding: encoding}, nil
}

// CountTokens returns the number of tokens in the given text.
func (c *Counter) CountTokens(text string) int {
	if c == nil || c.encoding == nil {
		return len(text) / 4 // rough fallback estimate
	}
	tokens := c.encoding.Encode(text, nil, nil)
	return len(tokens)
}

// CountMessageTokens estimates tokens for a list of messages.
func (c *Counter) CountMessageTokens(messages []types.Message) int {
	if c == nil || c.encoding == nil {
		return 0
	}

	total := 0
	for _, msg := range messages {
		blocks := msg.ContentBlocks()
		for _, block := range blocks {
			switch block.Type {
			case "text", "thinking":
				total += c.CountTokens(block.Text)
				if block.Thinking != "" {
					total += c.CountTokens(block.Thinking)
				}
			case "tool_use":
				total += c.CountTokens(block.Name)
				if len(block.Input) > 0 {
					total += c.CountTokens(string(block.Input))
				}
			case "tool_result":
				total += c.CountTokens(block.TextContent())
			}
		}
	}

	return total
}
