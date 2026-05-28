// Package proxy implements the HTTP proxy server.
package proxy

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/nilparra-dev/opencode-go-cc/internal/config"
	"github.com/nilparra-dev/opencode-go-cc/internal/transform"
	"github.com/nilparra-dev/opencode-go-cc/pkg/types"
)

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (s *Server) handleMessages(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx := r.Context()

	// Parse Anthropic request
	var anthropicReq types.MessageRequest
	if err := json.NewDecoder(r.Body).Decode(&anthropicReq); err != nil {
		slog.Error("failed to decode request", "error", err)
		writeError(w, http.StatusBadRequest, "invalid_request_error", "Failed to decode request body")
		return
	}

	// Count tokens for routing
	tokenCount := 0
	if s.tokenCounter != nil {
		tokenCount = s.tokenCounter.CountMessageTokens(anthropicReq.Messages)
	}

	// Select model
	isStreaming := anthropicReq.Stream != nil && *anthropicReq.Stream
	result, err := s.router.Select(anthropicReq.Messages, tokenCount, anthropicReq.Model, isStreaming)
	if err != nil {
		slog.Error("failed to select model", "error", err)
		writeError(w, http.StatusInternalServerError, "api_error", "Failed to select model")
		return
	}

	slog.Info("routing request",
		"scenario", result.Scenario,
		"model", result.Primary.ModelID,
		"streaming", isStreaming,
	)

	// Try primary model and fallbacks
	chain := result.GetModelChain()
	var lastErr error

	for _, model := range chain {
		openaiReq, err := s.reqTransformer.TransformRequest(&anthropicReq, model)
		if err != nil {
			lastErr = err
			continue
		}

		if isStreaming {
			err = s.handleStreaming(ctx, w, openaiReq, anthropicReq.Model, model)
		} else {
			err = s.handleNonStreaming(ctx, w, openaiReq, anthropicReq.Model, model)
		}

		if err == nil {
			return // Success
		}

		lastErr = err
		slog.Warn("model request failed, trying fallback", "model", model.ModelID, "error", err)
	}

	slog.Error("all models failed", "error", lastErr)
	writeError(w, http.StatusInternalServerError, "api_error", "All models in the fallback chain failed")
}

func (s *Server) handleNonStreaming(ctx context.Context, w http.ResponseWriter, openaiReq *types.ChatCompletionRequest, originalModel string, model config.ModelConfig) error {
	openaiResp, err := s.ocClient.SendRequest(ctx, openaiReq)
	if err != nil {
		return err
	}

	anthropicResp, err := s.respTransformer.TransformResponse(openaiResp, originalModel)
	if err != nil {
		return err
	}

	w.Header().Set("Content-Type", "application/json")
	return json.NewEncoder(w).Encode(anthropicResp)
}

func (s *Server) handleStreaming(ctx context.Context, w http.ResponseWriter, openaiReq *types.ChatCompletionRequest, originalModel string, model config.ModelConfig) error {
	reader, err := s.ocClient.SendStreamRequest(ctx, openaiReq)
	if err != nil {
		return err
	}
	defer reader.Close()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}

	streamTransformer := transform.NewStreamTransformer(originalModel, w)
	return streamTransformer.Transform(reader)
}

func (s *Server) handleCountTokens(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Messages []types.Message `json:"messages"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request_error", "Failed to decode request")
		return
	}

	count := 0
	if s.tokenCounter != nil {
		count = s.tokenCounter.CountMessageTokens(req.Messages)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]int{"input_tokens": count})
}

func (s *Server) handleModels(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	models := []map[string]interface{}{
		{"id": "claude-3-opus-20240229", "display_name": "Claude 3 Opus"},
		{"id": "claude-3-sonnet-20240229", "display_name": "Claude 3 Sonnet"},
		{"id": "claude-3-haiku-20240307", "display_name": "Claude 3 Haiku"},
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"data": models,
	})
}

func writeError(w http.ResponseWriter, statusCode int, errType, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"type": "error",
		"error": map[string]interface{}{
			"type":    errType,
			"message": message,
		},
	})
}
