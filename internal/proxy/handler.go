// Package proxy implements the HTTP proxy server.
package proxy

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"sort"

	"github.com/nilparra-dev/opencode-go-cc/internal/client"
	"github.com/nilparra-dev/opencode-go-cc/internal/config"
)

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
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
