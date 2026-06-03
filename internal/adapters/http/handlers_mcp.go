package http

import (
	"encoding/json"
	"log"
	"net/http"

	"llmplaceholder/internal/core/models"
	"llmplaceholder/internal/core/registry"
)

func HandleMCPMessage(w http.ResponseWriter, r *http.Request) {
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

	// 2. Dispatch the JSON-RPC Method
	switch req.Method {
	case "tools/call":
		// Parse the incoming tool arguments
		var params struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal(req.Params, &params); err != nil {
			resp.Error = map[string]interface{}{"code": -32602, "message": "Invalid params"}
			break
		}

		// 3. Consult the Core Domain Registry for high-fidelity tool data
		scenario := registry.MatchTool(params.Name)

		// 4. Format according to MCP Specification
		resp.Result = map[string]interface{}{
			"content": []map[string]interface{}{
				{
					"type": "text",
					"text": "Successfully retrieved mock dataset.",
				},
				{
					"type": "json",
					"data": scenario.MCPToolData,
				},
			},
		}

	case "tools/list":
		// Return a generic list based on our registry
		resp.Result = map[string]interface{}{
			"tools": []map[string]string{
				{"name": "get_invoice_ledger", "description": "Fetches overdue invoices."},
				{"name": "fetch_pod_metrics", "description": "Fetches Kubernetes RAM/CPU data."},
			},
		}

	default:
		resp.Error = map[string]interface{}{"code": -32601, "message": "Method not found"}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
