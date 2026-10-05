package proxy

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"
)

const (
	maxRequestBody = 256 << 20 // generous: requests can embed images and long histories

	// Placeholder credentials written by `occb on --exclusive` / `occb run --exclusive`.
	// A request carrying one has no valid Anthropic credential to forward.
	placeholderAuthToken = "unused"
	placeholderAPIKey    = "occb-proxy"
)

func newUpstreamTransport() http.RoundTripper {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.ResponseHeaderTimeout = 5 * time.Minute
	return transport
}

// hasPlaceholderCredential reports whether the request was authenticated with
// the dummy credential used in exclusive OpenCode mode.
func hasPlaceholderCredential(r *http.Request) bool {
	if strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ") == placeholderAuthToken {
		return true
	}
	return r.Header.Get("x-api-key") == placeholderAPIKey
}

// passthroughEnabled reports whether a request for a Claude model should go to Anthropic.
func (s *Server) passthroughEnabled(r *http.Request) bool {
	cfg := s.cfg.Get()
	return cfg.Anthropic.Passthrough && cfg.Anthropic.BaseURL != "" && !hasPlaceholderCredential(r)
}

// readBody reads and restores nothing: the caller owns the returned bytes.
func readBody(w http.ResponseWriter, r *http.Request) ([]byte, error) {
	if r.Body == nil {
		return nil, nil
	}
	return io.ReadAll(http.MaxBytesReader(w, r.Body, maxRequestBody))
}

// forwardToAnthropic proxies the request to Anthropic unchanged (apart from the
// body, which the caller may have rewritten). Headers - including the user's
// OAuth bearer or API key and the anthropic-beta flags - are forwarded as-is,
// and streaming responses are flushed as they arrive.
func (s *Server) forwardToAnthropic(w http.ResponseWriter, r *http.Request, body []byte) {
	target, err := url.Parse(s.cfg.Get().Anthropic.BaseURL)
	if err != nil || target.Host == "" {
		writeError(w, http.StatusBadGateway, "api_error", "Invalid anthropic.base_url in occb config")
		return
	}

	r.Body = io.NopCloser(bytes.NewReader(body))
	r.ContentLength = int64(len(body))

	proxy := &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.SetURL(target)
			pr.Out.Host = target.Host
		},
		Transport:     s.upstream,
		FlushInterval: -1,
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			slog.Error("anthropic passthrough failed", "path", r.URL.Path, "error", err)
			writeError(w, http.StatusBadGateway, "api_error", "Failed to reach Anthropic: "+err.Error())
		},
	}
	proxy.ServeHTTP(w, r)
}

// handleFallthrough forwards any endpoint occb does not implement to Anthropic.
func (s *Server) handleFallthrough(w http.ResponseWriter, r *http.Request) {
	if !s.passthroughEnabled(r) {
		writeError(w, http.StatusNotFound, "not_found_error", "Unsupported endpoint: "+r.URL.Path)
		return
	}

	body, err := readBody(w, r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request_error", "Failed to read request body")
		return
	}
	s.forwardToAnthropic(w, r, body)
}

// stripUnsignedThinking removes thinking blocks that have no signature from
// assistant messages. Those blocks come from non-Claude models earlier in the
// conversation; Anthropic rejects them when the user switches back to a Claude
// model. If anything was removed the request's own "thinking" setting is also
// dropped, since Anthropic requires thinking blocks on tool-use turns whenever
// thinking is enabled.
func stripUnsignedThinking(body []byte) ([]byte, bool) {
	if !bytes.Contains(body, []byte(`"thinking"`)) {
		return body, false
	}

	var top map[string]json.RawMessage
	if err := json.Unmarshal(body, &top); err != nil {
		return body, false
	}

	var messages []map[string]json.RawMessage
	if err := json.Unmarshal(top["messages"], &messages); err != nil {
		return body, false
	}

	changed := false
	kept := make([]map[string]json.RawMessage, 0, len(messages))

	for _, msg := range messages {
		var role string
		_ = json.Unmarshal(msg["role"], &role)

		var blocks []map[string]json.RawMessage
		if role != "assistant" || json.Unmarshal(msg["content"], &blocks) != nil {
			kept = append(kept, msg)
			continue
		}

		filtered := make([]map[string]json.RawMessage, 0, len(blocks))
		for _, block := range blocks {
			var blockType, signature string
			_ = json.Unmarshal(block["type"], &blockType)
			_ = json.Unmarshal(block["signature"], &signature)

			if blockType == "thinking" && signature == "" {
				changed = true
				continue
			}
			filtered = append(filtered, block)
		}

		if len(filtered) == len(blocks) {
			kept = append(kept, msg)
			continue
		}
		if len(filtered) == 0 {
			continue // nothing left of this message
		}

		content, err := json.Marshal(filtered)
		if err != nil {
			return body, false
		}
		msg["content"] = content
		kept = append(kept, msg)
	}

	if !changed {
		return body, false
	}

	rewritten, err := json.Marshal(kept)
	if err != nil {
		return body, false
	}
	top["messages"] = rewritten
	delete(top, "thinking")

	out, err := json.Marshal(top)
	if err != nil {
		return body, false
	}
	return out, true
}
