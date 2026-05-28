// Package client provides HTTP clients for upstream API providers.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
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

// ModelInfo represents a model entry from the OpenCode Go catalog.
type ModelInfo struct {
	ID          string
	DisplayName string
	Description string
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

// UsesAnthropicEndpoint returns true for models that use the Anthropic endpoint.
func (c *OpenCodeClient) UsesAnthropicEndpoint(modelID string) bool {
	return c.isAnthropicModel(modelID)
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

// SendAnthropicRequest sends a non-streaming Anthropic Messages request.
func (c *OpenCodeClient) SendAnthropicRequest(ctx context.Context, req *types.MessageRequest) (*types.MessageResponse, error) {
	if c.cfg.AnthropicBaseURL == "" {
		return nil, fmt.Errorf("anthropic base URL is not configured")
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.AnthropicBaseURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", c.apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

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

	var messageResp types.MessageResponse
	if err := json.Unmarshal(respBody, &messageResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &messageResp, nil
}

// SendAnthropicStreamRequest sends a streaming Anthropic Messages request.
func (c *OpenCodeClient) SendAnthropicStreamRequest(ctx context.Context, req *types.MessageRequest) (io.ReadCloser, error) {
	if c.cfg.AnthropicBaseURL == "" {
		return nil, fmt.Errorf("anthropic base URL is not configured")
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.AnthropicBaseURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", c.apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")
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

// ListModels fetches the upstream OpenCode Go catalog.
func (c *OpenCodeClient) ListModels(ctx context.Context) ([]ModelInfo, error) {
	modelsURL := strings.TrimSuffix(c.cfg.BaseURL, "/chat/completions") + "/models"

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, modelsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

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

	var payload struct {
		Data []struct {
			ID          string `json:"id"`
			DisplayName string `json:"display_name"`
			Description string `json:"description"`
		} `json:"data"`
	}

	if err := json.Unmarshal(respBody, &payload); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	models := make([]ModelInfo, 0, len(payload.Data))
	for _, model := range payload.Data {
		displayName := model.DisplayName
		if displayName == "" {
			displayName = model.ID
		}

		models = append(models, ModelInfo{
			ID:          model.ID,
			DisplayName: displayName,
			Description: model.Description,
		})
	}

	return models, nil
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
