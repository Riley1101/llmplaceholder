package http

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"llmplaceholder/internal/core/models"
	"llmplaceholder/internal/core/registry"
	"llmplaceholder/internal/db"
)

// HandleMCPMessage handles POST /mcp/message.
// Supports both plain JSON and SSE responses (streamable HTTP transport, spec 2025-03-26).
// When the client sends Accept: text/event-stream the JSON-RPC response is wrapped as an SSE event.
func HandleMCPMessage(dbManager *db.TenantDBManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := r.Context().Value(TenantIDKey).(string)
		log.Printf("[MCP] POST /mcp/message tenant=%s\n", tenantID)

		var req models.JSONRPCRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid JSON-RPC", http.StatusBadRequest)
			return
		}

		resp := models.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID}
		tenantScenarios, _ := dbManager.GetScenariosForTenant(tenantID)

		switch req.Method {
		case "tools/call":
			var params struct {
				Name string `json:"name"`
			}
			if err := json.Unmarshal(req.Params, &params); err != nil {
				resp.Error = map[string]interface{}{"code": -32602, "message": "Invalid params"}
				break
			}
			scenario := registry.MatchToolForTenant(params.Name, tenantScenarios)
			toolData := scenario.MCPToolData
			if scenario.StateKey != "" {
				if state, err := dbManager.ReadState(tenantID); err == nil {
					if node, ok := state[scenario.StateKey]; ok {
						toolData = node
					}
				}
			}
			resp.Result = map[string]interface{}{
				"content": []map[string]interface{}{
					{"type": "text", "text": "Successfully retrieved mock dataset."},
					{"type": "json", "data": toolData},
				},
			}

		case "tools/list":
			includeGlobal := true
			if settings, err := dbManager.ReadSettings(tenantID); err == nil {
				if v, ok := settings["include_global_tools"]; ok {
					if b, ok := v.(bool); ok {
						includeGlobal = b
					}
				}
			}
			resp.Result = map[string]interface{}{
				"tools": registry.ListToolsForTenant(tenantScenarios, includeGlobal),
			}

		case "resources/list":
			resp.Result = map[string]interface{}{"resources": []interface{}{}}

		case "prompts/list":
			resp.Result = map[string]interface{}{"prompts": []interface{}{}}

		case "initialize":
			resp.Result = map[string]interface{}{
				"protocolVersion": "2025-03-26",
				"capabilities": map[string]interface{}{
					"tools":     map[string]bool{"listChanged": false},
					"resources": map[string]bool{"listChanged": false},
				},
				"serverInfo": map[string]string{
					"name":    "llmplaceholder",
					"version": "1.0.0",
				},
			}

		default:
			resp.Error = map[string]interface{}{"code": -32601, "message": "Method not found"}
		}

		wantsSSE := strings.Contains(r.Header.Get("Accept"), "text/event-stream")
		if wantsSSE {
			flusher, ok := w.(http.Flusher)
			if !ok {
				http.Error(w, "SSE not supported", http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("Connection", "keep-alive")
			data, _ := json.Marshal(resp)
			fmt.Fprintf(w, "event: message\ndata: %s\n\n", data)
			flusher.Flush()
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

// HandleMCPSSE handles GET /mcp/sse.
// Implements the legacy HTTP+SSE transport (spec 2024-11-05):
// establishes a persistent SSE connection, emits an endpoint event with the POST URL,
// then keeps the connection alive with periodic pings until the client disconnects.
func HandleMCPSSE(dbManager *db.TenantDBManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := r.Context().Value(TenantIDKey).(string)

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "SSE not supported", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		sessionID := fmt.Sprintf("%d", time.Now().UnixNano())
		postURL := fmt.Sprintf("/mcp/message?sessionId=%s", sessionID)

		fmt.Fprintf(w, "event: endpoint\ndata: %s\n\n", postURL)
		flusher.Flush()
		log.Printf("[MCP SSE] session=%s tenant=%s connected\n", sessionID, tenantID)

		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-r.Context().Done():
				log.Printf("[MCP SSE] session=%s disconnected\n", sessionID)
				return
			case <-ticker.C:
				fmt.Fprintf(w, ": ping\n\n")
				flusher.Flush()
			}
		}
	}
}
