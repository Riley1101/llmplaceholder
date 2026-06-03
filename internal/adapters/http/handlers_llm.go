package http

import (
	"encoding/json"
	"log"
	"net/http"

	"llmplaceholder/internal/chronos"
	"llmplaceholder/internal/core/models"
	"llmplaceholder/internal/core/registry"
)

// HandleOpenAI translates the 1-to-1 official OpenAI payload
func HandleOpenAI(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Context().Value(TenantIDKey).(string)
	log.Printf("[OpenAI Adapter] Processing stream for Tenant: %s\n", tenantID)

	var req models.OpenAIChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	// 1. Extract prompt
	prompt := ""
	if len(req.Messages) > 0 {
		prompt = req.Messages[len(req.Messages)-1].Content
	}

	// 2. Pass to Core Domain (No HTTP logic here)
	scenario := registry.MatchIntent(prompt)

	// 3. Setup Egress Stream
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	// 4. Pass to Physics Engine
	chronos.StreamOpenAI(w, flusher, req.Model, scenario.FullResponse)
}
