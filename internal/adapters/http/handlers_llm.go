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

// denyPrivateTenant returns true (and writes the error) if the tenant is private and the caller
// is unauthenticated or is not the owner. Allows access when the tenant doesn't exist yet
// (it will be auto-provisioned as global by ReadState).
func denyPrivateTenant(w http.ResponseWriter, r *http.Request, dbManager *db.TenantDBManager, tenantID string) bool {
	ownerID, isGlobal, err := dbManager.TenantOwner(tenantID)
	if err != nil || isGlobal {
		return false // unknown tenant → treat as global; global → open
	}
	user := UserFromContext(r)
	if user == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]string{
				"message": "private tenant requires authentication — add Authorization: Bearer <token>",
				"type":    "authentication_error",
				"code":    "unauthorized",
			},
		})
		return true
	}
	if ownerID != user.ID {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]string{
				"message": "not your tenant",
				"type":    "permission_error",
				"code":    "forbidden",
			},
		})
		return true
	}
	return false
}

func HandleAnthropic(dbManager *db.TenantDBManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := r.Context().Value(TenantIDKey).(string)
		if denyPrivateTenant(w, r, dbManager, tenantID) {
			return
		}
		log.Printf("[Anthropic Adapter] Processing request for Tenant: %s\n", tenantID)

		var req models.AnthropicMessageRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}

		prompt := ""
		if len(req.Messages) > 0 {
			prompt = req.Messages[len(req.Messages)-1].Content
		}

		tenantScenarios, _ := dbManager.GetScenariosForTenant(tenantID)
		scenario := registry.MatchIntent(prompt, tenantScenarios)

		if !req.Stream {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			json.NewEncoder(w).Encode(chronos.BuildAnthropic(req.Model, scenario.FullResponse))
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

		chronos.StreamAnthropic(w, flusher, req.Model, scenario.FullResponse)
	}
}

func HandleOpenAI(dbManager *db.TenantDBManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := r.Context().Value(TenantIDKey).(string)
		if denyPrivateTenant(w, r, dbManager, tenantID) {
			return
		}
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
		scenario := registry.MatchIntent(prompt, tenantScenarios)

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
