// Package client provides HTTP clients for upstream API providers.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/nilparra-dev/opencode-go-cc/internal/config"
	"github.com/nilparra-dev/opencode-go-cc/pkg/types"
)

// OpenCodeClient handles communication with the OpenCode Go API.
type OpenCodeClient struct {
	cfg        *config.OpenCodeGoConfig
	httpClient *http.Client
	apiKey     string
}

// NewOpenCodeClient creates a new OpenCode Go API client.
func NewOpenCodeClient(cfg *config.OpenCodeGoConfig, apiKey string) *OpenCodeClient {
	timeout := time.Duration(cfg.TimeoutMs) * time.Millisecond
	if timeout == 0 {
		timeout = 5 * time.Minute
	}

	return &OpenCodeClient{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: timeout,
		},
		apiKey: apiKey,
	}
}

// SendRequest sends a non-streaming chat completion request.
func (c *OpenCodeClient) SendRequest(ctx context.Context, req *types.ChatCompletionRequest) (*types.ChatCompletionResponse, error) {
	baseURL := c.cfg.BaseURL
	if c.isAnthropicModel(req.Model) && c.cfg.AnthropicBaseURL != "" {
		baseURL = c.cfg.AnthropicBaseURL
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("upstream error %d: %s", resp.StatusCode, string(respBody))
	}

	var chatResp types.ChatCompletionResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &chatResp, nil
}

// SendStreamRequest sends a streaming chat completion request and returns the response body reader.
func (c *OpenCodeClient) SendStreamRequest(ctx context.Context, req *types.ChatCompletionRequest) (io.ReadCloser, error) {
	baseURL := c.cfg.BaseURL
	if c.isAnthropicModel(req.Model) && c.cfg.AnthropicBaseURL != "" {
		baseURL = c.cfg.AnthropicBaseURL
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	httpReq.Header.Set("Accept", "text/event-stream")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("upstream error %d: %s", resp.StatusCode, string(respBody))
	}

	return resp.Body, nil
}

// isAnthropicModel returns true for models that use the Anthropic endpoint.
func (c *OpenCodeClient) isAnthropicModel(modelID string) bool {
	switch modelID {
	case "minimax-m2.7", "minimax-m2.5", "qwen3.7-max", "qwen3.6-plus", "qwen3.5-plus":
		return true
	default:
		return false
	}
}
