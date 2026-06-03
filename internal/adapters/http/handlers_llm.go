package http

import (
	"encoding/json"
	"log"
	"net/http"

	"llmplaceholder/internal/chronos"
	"llmplaceholder/internal/core/models"
	"llmplaceholder/internal/core/registry"
	"llmplaceholder/internal/db"
)

func HandleOpenAI(dbManager *db.TenantDBManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := r.Context().Value(TenantIDKey).(string)
		log.Printf("[OpenAI Adapter] Processing stream for Tenant: %s\n", tenantID)

		var req models.OpenAIChatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}

		prompt := ""
		if len(req.Messages) > 0 {
			prompt = req.Messages[len(req.Messages)-1].Content
		}

		tenantScenarios, _ := dbManager.GetScenariosForTenant(tenantID)
		scenario := registry.MatchIntentForTenant(prompt, tenantScenarios)

		if !req.Stream {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			json.NewEncoder(w).Encode(chronos.BuildOpenAI(req.Model, scenario.FullResponse))
			return
		}

		w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
			return
		}

		chronos.StreamOpenAI(w, flusher, req.Model, scenario.FullResponse)
	}
}
