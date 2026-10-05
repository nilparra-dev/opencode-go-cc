package proxy

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/nilparra-dev/opencode-go-cc/internal/config"
)

// newTestProxy starts occb in front of fake Anthropic and OpenCode upstreams.
func newTestProxy(t *testing.T, anthropic, opencode http.Handler, mutate func(*config.Config)) *httptest.Server {
	t.Helper()

	anthropicSrv := httptest.NewServer(anthropic)
	t.Cleanup(anthropicSrv.Close)
	opencodeSrv := httptest.NewServer(opencode)
	t.Cleanup(opencodeSrv.Close)

	cfg := config.DefaultConfig()
	cfg.APIKey = "oc-key"
	cfg.Anthropic.BaseURL = anthropicSrv.URL
	cfg.OpenCodeGo.BaseURL = opencodeSrv.URL + "/v1/chat/completions"
	cfg.OpenCodeGo.AnthropicBaseURL = opencodeSrv.URL + "/v1/messages"
	if mutate != nil {
		mutate(cfg)
	}

	srv, err := NewServer(config.NewAtomicConfig(cfg, ""))
	if err != nil {
		t.Fatal(err)
	}
	srv.tokenCounter = nil // avoid loading the tiktoken vocabulary in tests

	front := httptest.NewServer(srv.httpServer.Handler)
	t.Cleanup(front.Close)
	return front
}

func post(t *testing.T, url string, body string, headers map[string]string) *http.Response {
	t.Helper()

	req, err := http.NewRequest(http.MethodPost, url, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func readAll(t *testing.T, resp *http.Response) string {
	t.Helper()
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func failHandler(t *testing.T, name string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected request to %s: %s %s", name, r.Method, r.URL.Path)
		http.Error(w, "unexpected", http.StatusTeapot)
	})
}

func TestClaudeModelsArePassedThroughToAnthropicWithOriginalCredentials(t *testing.T) {
	var gotAuth, gotBeta, gotBody, gotPath string
	anthropic := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotBeta = r.Header.Get("anthropic-beta")
		gotPath = r.URL.Path
		body, _ := io.ReadAll(r.Body)
		gotBody = string(body)

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"msg_real","type":"message"}`))
	})

	front := newTestProxy(t, anthropic, failHandler(t, "opencode"), nil)

	body := `{"model":"claude-sonnet-4-5","max_tokens":10,"messages":[{"role":"user","content":"hi"}]}`
	resp := post(t, front.URL+"/v1/messages", body, map[string]string{
		"Authorization":  "Bearer oauth-token",
		"anthropic-beta": "oauth-2025-04-20",
	})

	if got := readAll(t, resp); got != `{"id":"msg_real","type":"message"}` {
		t.Fatalf("response body = %s", got)
	}
	if gotAuth != "Bearer oauth-token" || gotBeta != "oauth-2025-04-20" {
		t.Fatalf("credentials not forwarded: auth=%q beta=%q", gotAuth, gotBeta)
	}
	if gotPath != "/v1/messages" || gotBody != body {
		t.Fatalf("upstream got path=%q body=%q", gotPath, gotBody)
	}
}

func TestClaudeStreamingPassthroughIsNotBuffered(t *testing.T) {
	release := make(chan struct{})
	anthropic := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("event: ping\ndata: {}\n\n"))
		w.(http.Flusher).Flush()
		<-release // the second event is held back until the client has read the first
		_, _ = w.Write([]byte("event: message_stop\ndata: {}\n\n"))
	})

	front := newTestProxy(t, anthropic, failHandler(t, "opencode"), nil)

	resp := post(t, front.URL+"/v1/messages", `{"model":"claude-haiku-4-5","stream":true,"messages":[]}`, nil)
	defer resp.Body.Close()

	buf := make([]byte, 64)
	n, err := resp.Body.Read(buf)
	if err != nil || !strings.Contains(string(buf[:n]), "ping") {
		t.Fatalf("first event not delivered while stream is open: n=%d err=%v", n, err)
	}
	close(release)
}

func TestOtherEndpointsFallThroughToAnthropic(t *testing.T) {
	var gotPath string
	anthropic := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path + "?" + r.URL.RawQuery
		_, _ = w.Write([]byte(`{"ok":true}`))
	})

	front := newTestProxy(t, anthropic, failHandler(t, "opencode"), nil)

	resp, err := http.Get(front.URL + "/v1/messages/batches?limit=5")
	if err != nil {
		t.Fatal(err)
	}
	_ = readAll(t, resp)

	if gotPath != "/v1/messages/batches?limit=5" {
		t.Fatalf("anthropic saw %q", gotPath)
	}
}

func TestOpenCodeModelStreamsAsAnthropicSSEAndFlushesLive(t *testing.T) {
	release := make(chan struct{})
	opencode := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer oc-key" {
			t.Errorf("OpenCode request missing API key")
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"Hi\"}}]}\n\n"))
		w.(http.Flusher).Flush()
		<-release
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n"))
	})

	front := newTestProxy(t, failHandler(t, "anthropic"), opencode, nil)

	resp := post(t, front.URL+"/v1/messages", `{"model":"kimi-k2.6","stream":true,"max_tokens":50,"messages":[{"role":"user","content":"hi"}]}`, nil)
	defer resp.Body.Close()

	// The text delta must reach the client before upstream finishes: this fails if
	// the logging middleware hides http.Flusher from the stream handler.
	var seen string
	buf := make([]byte, 4096)
	for !strings.Contains(seen, `"text":"Hi"`) {
		n, err := resp.Body.Read(buf)
		seen += string(buf[:n])
		if err != nil {
			t.Fatalf("stream ended before the first delta arrived: %v\n%s", err, seen)
		}
	}
	close(release)

	rest, _ := io.ReadAll(resp.Body)
	all := seen + string(rest)
	for _, want := range []string{"message_start", `"stop_reason":"end_turn"`, "message_stop"} {
		if !strings.Contains(all, want) {
			t.Errorf("stream is missing %q:\n%s", want, all)
		}
	}
}

func TestFallbackHappensOnlyBeforeResponseStarts(t *testing.T) {
	var calls atomic.Int32
	opencode := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Model string `json:"model"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)

		if calls.Add(1) == 1 {
			http.Error(w, "boom", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"1","choices":[{"message":{"content":"from fallback"},"finish_reason":"stop"}]}`))
	})

	front := newTestProxy(t, failHandler(t, "anthropic"), opencode, nil)

	resp := post(t, front.URL+"/v1/messages", `{"model":"kimi-k2.6","max_tokens":50,"messages":[{"role":"user","content":"hi"}]}`, nil)
	body := readAll(t, resp)

	if resp.StatusCode != http.StatusOK || !strings.Contains(body, "from fallback") {
		t.Fatalf("status=%d body=%s", resp.StatusCode, body)
	}
	if calls.Load() != 2 {
		t.Fatalf("upstream calls = %d, want 2 (primary + fallback)", calls.Load())
	}
}

func TestMidStreamFailureEmitsErrorEventInsteadOfASecondResponse(t *testing.T) {
	var calls atomic.Int32
	opencode := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"partial\"}}]}\n\n"))
		w.(http.Flusher).Flush()
		// Abort the connection mid-stream.
		panic(http.ErrAbortHandler)
	})

	front := newTestProxy(t, failHandler(t, "anthropic"), opencode, nil)

	resp := post(t, front.URL+"/v1/messages", `{"model":"kimi-k2.6","stream":true,"max_tokens":50,"messages":[{"role":"user","content":"hi"}]}`, nil)
	body := readAll(t, resp)

	if !strings.Contains(body, "event: error") {
		t.Fatalf("expected an SSE error event, got:\n%s", body)
	}
	if calls.Load() != 1 {
		t.Fatalf("upstream calls = %d, want 1: no fallback once the response started", calls.Load())
	}
}

func TestExclusiveModeCredentialIsNeverSentToAnthropic(t *testing.T) {
	opencode := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"id":"1","choices":[{"message":{"content":"routed to opencode"},"finish_reason":"stop"}]}`))
	})

	front := newTestProxy(t, failHandler(t, "anthropic"), opencode, nil)

	resp := post(t, front.URL+"/v1/messages", `{"model":"claude-haiku-4-5","max_tokens":50,"messages":[{"role":"user","content":"hi"}]}`,
		map[string]string{"Authorization": "Bearer unused"})
	body := readAll(t, resp)

	if !strings.Contains(body, "routed to opencode") {
		t.Fatalf("body = %s", body)
	}
}

func TestPassthroughCanBeDisabled(t *testing.T) {
	opencode := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"id":"1","choices":[{"message":{"content":"opencode"},"finish_reason":"stop"}]}`))
	})

	front := newTestProxy(t, failHandler(t, "anthropic"), opencode, func(c *config.Config) {
		c.Anthropic.Passthrough = false
	})

	resp := post(t, front.URL+"/v1/messages", `{"model":"claude-sonnet-4-5","max_tokens":50,"messages":[{"role":"user","content":"hi"}]}`, nil)
	if body := readAll(t, resp); !strings.Contains(body, "opencode") {
		t.Fatalf("body = %s", body)
	}
}

func TestStripUnsignedThinking(t *testing.T) {
	body := []byte(`{"model":"claude-sonnet-4-5","thinking":{"type":"enabled","budget_tokens":1024},"max_tokens":9999999999,"messages":[` +
		`{"role":"user","content":"go"},` +
		`{"role":"assistant","content":[{"type":"thinking","thinking":"from kimi"},{"type":"text","text":"ok"}]},` +
		`{"role":"assistant","content":[{"type":"thinking","thinking":"only unsigned"}]},` +
		`{"role":"assistant","content":[{"type":"thinking","thinking":"real","signature":"sig123"},{"type":"text","text":"keep"}]}` +
		`]}`)

	out, changed := stripUnsignedThinking(body)
	if !changed {
		t.Fatal("expected a change")
	}

	var got struct {
		Thinking  json.RawMessage `json:"thinking"`
		MaxTokens json.Number     `json:"max_tokens"`
		Messages  []struct {
			Content json.RawMessage `json:"content"`
		} `json:"messages"`
	}
	type block struct {
		Type      string `json:"type"`
		Signature string `json:"signature"`
	}
	blocksOf := func(i int) []block {
		var blocks []block
		if err := json.Unmarshal(got.Messages[i].Content, &blocks); err != nil {
			t.Fatalf("message %d content is not a block array: %v", i, err)
		}
		return blocks
	}
	dec := json.NewDecoder(strings.NewReader(string(out)))
	dec.UseNumber()
	if err := dec.Decode(&got); err != nil {
		t.Fatal(err)
	}

	if got.Thinking != nil {
		t.Errorf("thinking setting should be dropped when blocks were stripped, got %s", got.Thinking)
	}
	if got.MaxTokens.String() != "9999999999" {
		t.Errorf("unrelated numbers must survive untouched, got %s", got.MaxTokens)
	}
	// user + text-only assistant + signed assistant; the "only unsigned" message is gone.
	if len(got.Messages) != 3 {
		t.Fatalf("messages = %d, want 3: %s", len(got.Messages), out)
	}
	if blocks := blocksOf(1); len(blocks) != 1 || blocks[0].Type != "text" {
		t.Errorf("unsigned thinking not removed: %+v", blocks)
	}
	if blocks := blocksOf(2); len(blocks) != 2 || blocks[0].Signature != "sig123" {
		t.Errorf("signed thinking must be kept: %+v", blocks)
	}
}

func TestStripUnsignedThinkingLeavesCleanRequestsAlone(t *testing.T) {
	body := []byte(`{"model":"claude-sonnet-4-5","thinking":{"type":"enabled"},"messages":[{"role":"assistant","content":[{"type":"thinking","thinking":"x","signature":"s"}]}]}`)

	out, changed := stripUnsignedThinking(body)
	if changed || string(out) != string(body) {
		t.Fatalf("request without unsigned thinking must be returned untouched, changed=%v", changed)
	}
}
