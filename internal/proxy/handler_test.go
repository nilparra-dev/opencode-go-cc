package proxy

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/nilparra-dev/opencode-go-cc/internal/config"
)

func TestHandleModelsReturnsUniqueSortedConfiguredModels(t *testing.T) {
	t.Parallel()

	srv := &Server{
		cfg: config.NewAtomicConfig(&config.Config{
			Models: map[string]config.ModelConfig{
				"default": {ModelID: "kimi-k2.6"},
				"fast":    {ModelID: "qwen3.6-plus"},
			},
			Fallbacks: map[string][]config.ModelConfig{
				"default": {
					{ModelID: "mimo-v2.5-pro"},
					{ModelID: "qwen3.6-plus"},
				},
				"think": {
					{ModelID: "glm-5"},
				},
			},
		}, ""),
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	rec := httptest.NewRecorder()

	srv.handleModels(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status code: got %d want %d", rec.Code, http.StatusOK)
	}

	var resp struct {
		Data []struct {
			ID          string `json:"id"`
			DisplayName string `json:"display_name"`
			Description string `json:"description"`
		} `json:"data"`
	}

	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	got := make([]string, 0, len(resp.Data))
	for _, model := range resp.Data {
		if model.DisplayName != model.ID {
			t.Fatalf("display_name mismatch for %q: got %q", model.ID, model.DisplayName)
		}
		if model.Description == "" {
			t.Fatalf("description missing for %q", model.ID)
		}
		got = append(got, model.ID)
	}

	want := []string{"glm-5", "kimi-k2.6", "mimo-v2.5-pro", "qwen3.6-plus"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected model list: got %v want %v", got, want)
	}
}