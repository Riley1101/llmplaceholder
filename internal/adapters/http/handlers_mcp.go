package http

import (
	"encoding/json"
	"log"
	"net/http"

	"llmplaceholder/internal/core/models"
	"llmplaceholder/internal/core/registry"
	"llmplaceholder/internal/db"
)

func HandleMCPMessage(dbManager *db.TenantDBManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := r.Context().Value(TenantIDKey).(string)
		log.Printf("[MCP Adapter] Processing RPC for Tenant: %s\n", tenantID)

		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req models.JSONRPCRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid JSON-RPC", http.StatusBadRequest)
			return
		}

		resp := models.JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
		}

		switch req.Method {
		case "tools/call":
			var params struct {
				Name string `json:"name"`
			}
			if err := json.Unmarshal(req.Params, &params); err != nil {
				resp.Error = map[string]interface{}{"code": -32602, "message": "Invalid params"}
				break
			}

			scenario := registry.MatchTool(params.Name)

			// Use tenant-specific data when available; fall back to global mock
			toolData := scenario.MCPToolData
			if scenario.StateKey != "" {
				if tenantState, err := dbManager.ReadState(tenantID); err == nil {
					if node, ok := tenantState[scenario.StateKey]; ok {
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
			resp.Result = map[string]interface{}{
				"tools": registry.ListTools(),
			}

		default:
			resp.Error = map[string]interface{}{"code": -32601, "message": "Method not found"}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}
