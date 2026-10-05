package proxy

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/nilparra-dev/opencode-go-cc/internal/config"
	"github.com/nilparra-dev/opencode-go-cc/internal/router"
	"github.com/nilparra-dev/opencode-go-cc/internal/transform"
	"github.com/nilparra-dev/opencode-go-cc/pkg/types"
)

func (s *Server) handleMessages(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx := r.Context()

	body, err := readBody(w, r)
	if err != nil {
		slog.Error("failed to read request", "error", err)
		writeError(w, http.StatusBadRequest, "invalid_request_error", "Failed to read request body")
		return
	}

	// Official Claude models bypass the translation layer entirely.
	if model := peekModel(body); router.IsClaudeModel(model) && s.passthroughEnabled(r) {
		if rewritten, changed := stripUnsignedThinking(body); changed {
			slog.Info("dropped unsigned thinking blocks from earlier OpenCode turns")
			body = rewritten
		}
		slog.Info("passthrough to anthropic", "model", model)
		s.forwardToAnthropic(w, r, body)
		return
	}

	// Parse Anthropic request
	var anthropicReq types.MessageRequest
	if err := json.Unmarshal(body, &anthropicReq); err != nil {
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

	// Try primary model and fallbacks, skipping models whose circuit is open.
	chain := s.healthyChain(result.GetModelChain())
	var lastErr error

	for _, model := range chain {
		started, err := s.tryModel(ctx, w, &anthropicReq, model, isStreaming)
		if err == nil {
			s.breaker.RecordSuccess(model.ModelID)
			return
		}

		if ctx.Err() != nil {
			return // the client went away; nothing to retry
		}

		s.breaker.RecordFailure(model.ModelID)

		if started {
			// Headers are already out, so another model cannot take over this response.
			slog.Error("stream failed mid-response", "model", model.ModelID, "error", err)
			writeSSEError(w, "Upstream stream interrupted: "+err.Error())
			return
		}

		lastErr = err
		slog.Warn("model request failed, trying fallback", "model", model.ModelID, "error", err)
	}

	slog.Error("all models failed", "error", lastErr)
	writeError(w, http.StatusBadGateway, "api_error", "All models in the fallback chain failed")
}

// healthyChain drops models with an open circuit. If every model is open it
// returns the full chain so the request still gets an attempt.
func (s *Server) healthyChain(chain []config.ModelConfig) []config.ModelConfig {
	healthy := make([]config.ModelConfig, 0, len(chain))
	for _, model := range chain {
		if s.breaker.CanTry(model.ModelID) {
			healthy = append(healthy, model)
		}
	}
	if len(healthy) == 0 {
		return chain
	}
	return healthy
}

// tryModel sends the request to one upstream model. started reports whether any
// response bytes were already written to the client (in which case no fallback
// is possible).
func (s *Server) tryModel(ctx context.Context, w http.ResponseWriter, req *types.MessageRequest, model config.ModelConfig, isStreaming bool) (started bool, err error) {
	if s.ocClient.UsesAnthropicEndpoint(model.ModelID) {
		upstreamReq := applyAnthropicModelOverrides(req, model)
		if isStreaming {
			return s.handleAnthropicStreaming(ctx, w, upstreamReq)
		}
		return false, s.handleAnthropicNonStreaming(ctx, w, upstreamReq, req.Model)
	}

	openaiReq, err := s.reqTransformer.TransformRequest(req, model)
	if err != nil {
		return false, err
	}

	if isStreaming {
		return s.handleStreaming(ctx, w, openaiReq, req.Model)
	}
	return false, s.handleNonStreaming(ctx, w, openaiReq, req.Model)
}

// peekModel extracts the "model" field without decoding the whole request.
func peekModel(body []byte) string {
	var peek struct {
		Model string `json:"model"`
	}
	_ = json.Unmarshal(body, &peek)
	return peek.Model
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

func (s *Server) handleNonStreaming(ctx context.Context, w http.ResponseWriter, openaiReq *types.ChatCompletionRequest, originalModel string) error {
	openaiResp, err := s.ocClient.SendRequest(ctx, openaiReq)
	if err != nil {
		return err
	}

	anthropicResp, err := s.respTransformer.TransformResponse(openaiResp, originalModel)
	if err != nil {
		return err
	}

	return writeJSON(w, anthropicResp)
}

func (s *Server) handleAnthropicNonStreaming(ctx context.Context, w http.ResponseWriter, req *types.MessageRequest, originalModel string) error {
	resp, err := s.ocClient.SendAnthropicRequest(ctx, req)
	if err != nil {
		return err
	}

	if originalModel != "" {
		resp.Model = originalModel
	}

	return writeJSON(w, resp)
}

// writeJSON marshals first so a marshalling failure can still fall back to another model.
func writeJSON(w http.ResponseWriter, v interface{}) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}

	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(data)
	return nil
}

func (s *Server) handleStreaming(ctx context.Context, w http.ResponseWriter, openaiReq *types.ChatCompletionRequest, originalModel string) (bool, error) {
	reader, err := s.ocClient.SendStreamRequest(ctx, openaiReq)
	if err != nil {
		return false, err
	}
	defer reader.Close()

	startSSE(w)

	streamTransformer := transform.NewStreamTransformer(originalModel, w)
	return true, streamTransformer.Transform(reader)
}

func (s *Server) handleAnthropicStreaming(ctx context.Context, w http.ResponseWriter, req *types.MessageRequest) (bool, error) {
	reader, err := s.ocClient.SendAnthropicStreamRequest(ctx, req)
	if err != nil {
		return false, err
	}
	defer reader.Close()

	startSSE(w)

	flusher, _ := w.(http.Flusher)
	buffered := bufio.NewReader(reader)
	for {
		chunk, readErr := buffered.ReadBytes('\n')
		if len(chunk) > 0 {
			if _, writeErr := w.Write(chunk); writeErr != nil {
				return true, writeErr
			}
			if flusher != nil {
				flusher.Flush()
			}
		}

		if readErr != nil {
			if readErr == io.EOF {
				return true, nil
			}
			return true, readErr
		}
	}
}

// startSSE sends the headers of a server-sent-events response.
func startSSE(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}

// writeSSEError reports a failure inside an already-started event stream.
func writeSSEError(w http.ResponseWriter, message string) {
	data, _ := json.Marshal(map[string]interface{}{
		"type": "error",
		"error": map[string]interface{}{
			"type":    "api_error",
			"message": message,
		},
	})

	_, _ = fmt.Fprintf(w, "event: error\ndata: %s\n\n", data)
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}

func (s *Server) handleCountTokens(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := readBody(w, r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request_error", "Failed to read request body")
		return
	}

	// Claude models are counted by Anthropic itself.
	if router.IsClaudeModel(peekModel(body)) && s.passthroughEnabled(r) {
		if rewritten, changed := stripUnsignedThinking(body); changed {
			body = rewritten
		}
		s.forwardToAnthropic(w, r, body)
		return
	}

	var req struct {
		Messages []types.Message `json:"messages"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
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
