// Package proxy implements the HTTP proxy server.
package proxy

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"sort"

	"github.com/nilparra-dev/opencode-go-cc/internal/client"
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
		if s.ocClient.UsesAnthropicEndpoint(model.ModelID) {
			anthropicUpstreamReq := applyAnthropicModelOverrides(&anthropicReq, model)

			if isStreaming {
				err = s.handleAnthropicStreaming(ctx, w, anthropicUpstreamReq)
			} else {
				err = s.handleAnthropicNonStreaming(ctx, w, anthropicUpstreamReq, anthropicReq.Model)
			}

			if err == nil {
				return
			}

			lastErr = err
			slog.Warn("model request failed, trying fallback", "model", model.ModelID, "error", err)
			continue
		}

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

func applyAnthropicModelOverrides(req *types.MessageRequest, model config.ModelConfig) *types.MessageRequest {
	cloned := *req
	cloned.Model = model.ModelID

	if model.Temperature > 0 {
		temperature := model.Temperature
		cloned.Temperature = &temperature
	}

	if model.MaxTokens > 0 {
		cloned.MaxTokens = model.MaxTokens
	}

	return &cloned
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

func (s *Server) handleAnthropicNonStreaming(ctx context.Context, w http.ResponseWriter, req *types.MessageRequest, originalModel string) error {
	resp, err := s.ocClient.SendAnthropicRequest(ctx, req)
	if err != nil {
		return err
	}

	if originalModel != "" {
		resp.Model = originalModel
	}

	w.Header().Set("Content-Type", "application/json")
	return json.NewEncoder(w).Encode(resp)
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

func (s *Server) handleAnthropicStreaming(ctx context.Context, w http.ResponseWriter, req *types.MessageRequest) error {
	reader, err := s.ocClient.SendAnthropicStreamRequest(ctx, req)
	if err != nil {
		return err
	}
	defer reader.Close()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	flusher, _ := w.(http.Flusher)
	if flusher != nil {
		flusher.Flush()
	}

	buffered := bufio.NewReader(reader)
	for {
		chunk, readErr := buffered.ReadBytes('\n')
		if len(chunk) > 0 {
			if _, writeErr := w.Write(chunk); writeErr != nil {
				return writeErr
			}
			if flusher != nil {
				flusher.Flush()
			}
		}

		if readErr != nil {
			if readErr == io.EOF {
				return nil
			}
			return readErr
		}
	}
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

	models := configuredModels(s.cfg.Get())
	if s.ocClient != nil {
		if upstreamModels, err := s.ocClient.ListModels(r.Context()); err == nil {
			models = mergeModelInfos(models, upstreamModels)
		} else {
			slog.Warn("failed to fetch upstream model catalog, using configured models", "error", err)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"data": models,
	})
}

func mergeModelInfos(configured []map[string]interface{}, upstream []client.ModelInfo) []map[string]interface{} {
	seen := make(map[string]map[string]interface{})

	for _, model := range configured {
		id, _ := model["id"].(string)
		if id == "" {
			continue
		}
		seen[id] = model
	}

	for _, model := range upstream {
		description := model.Description
		if description == "" {
			description = "OpenCode Go model"
		}
		seen[model.ID] = map[string]interface{}{
			"id":           model.ID,
			"display_name": model.DisplayName,
			"description":  description,
		}
	}

	ids := make([]string, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	models := make([]map[string]interface{}, 0, len(ids))
	for _, id := range ids {
		models = append(models, seen[id])
	}

	return models
}

func configuredModels(cfg *config.Config) []map[string]interface{} {
	seen := make(map[string]struct{})
	ids := make([]string, 0, len(cfg.Models))

	addModel := func(modelID string) {
		if modelID == "" {
			return
		}
		if _, ok := seen[modelID]; ok {
			return
		}
		seen[modelID] = struct{}{}
		ids = append(ids, modelID)
	}

	for _, model := range cfg.Models {
		addModel(model.ModelID)
	}

	for _, fallbacks := range cfg.Fallbacks {
		for _, model := range fallbacks {
			addModel(model.ModelID)
		}
	}

	sort.Strings(ids)

	models := make([]map[string]interface{}, 0, len(ids))
	for _, modelID := range ids {
		models = append(models, map[string]interface{}{
			"id":           modelID,
			"display_name": modelID,
			"description":  "OpenCode Go model",
		})
	}

	return models
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
