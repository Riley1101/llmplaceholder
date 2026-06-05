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

// isOwner returns true if the authenticated user owns the given tenant.
func isOwner(r *http.Request, dbManager *db.TenantDBManager, tenantID string) bool {
	user := UserFromContext(r)
	if user == nil {
		return false
	}
	ownerID, _, err := dbManager.TenantOwner(tenantID)
	if err != nil {
		return false
	}
	return ownerID == user.ID
}

var mgmtTools = []map[string]interface{}{
	{
		"name":        "create_scenario",
		"description": "Create a draft scenario. Appears in dashboard for review before going active.",
		"inputSchema": map[string]interface{}{
			"type":     "object",
			"required": []string{"keywords", "response"},
			"properties": map[string]interface{}{
				"keywords": map[string]string{
					"type":        "string",
					"description": "Comma-separated keywords that trigger this scenario (e.g. 'invoice, billing')",
				},
				"response": map[string]string{
					"type":        "string",
					"description": "Mock response text to return when a prompt matches these keywords",
				},
			},
		},
	},
	{
		"name":        "list_scenarios",
		"description": "List scenarios for this tenant.",
		"inputSchema": map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"status": map[string]string{
					"type":        "string",
					"description": "Filter by status: 'active', 'draft', or omit for all",
				},
			},
		},
	},
	{
		"name":        "get_tenant",
		"description": "Get tenant info and scenario counts.",
		"inputSchema": map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		},
	},
}

func handleMgmtTool(dbManager *db.TenantDBManager, tenantID, tool string, rawParams json.RawMessage) interface{} {
	var args map[string]interface{}
	if rawParams != nil {
		json.Unmarshal(rawParams, &args)
	}
	if args == nil {
		args = map[string]interface{}{}
	}

	mcpText := func(s string) interface{} {
		return map[string]interface{}{
			"content": []map[string]string{{"type": "text", "text": s}},
		}
	}
	mcpErr := func(s string) interface{} {
		return map[string]interface{}{
			"content": []map[string]string{{"type": "text", "text": s}},
			"isError": true,
		}
	}
	strArg := func(key string) string {
		if v, ok := args[key]; ok {
			if s, ok := v.(string); ok {
				return strings.TrimSpace(s)
			}
		}
		return ""
	}

	switch tool {
	case "create_scenario":
		response := strArg("response")
		if response == "" {
			return mcpErr("response is required")
		}
		var keywords []string
		switch v := args["keywords"].(type) {
		case string:
			for _, k := range strings.Split(v, ",") {
				if k = strings.TrimSpace(k); k != "" {
					keywords = append(keywords, k)
				}
			}
		case []interface{}:
			for _, item := range v {
				if s, ok := item.(string); ok {
					if s = strings.TrimSpace(s); s != "" {
						keywords = append(keywords, s)
					}
				}
			}
		}
		if len(keywords) == 0 {
			return mcpErr("keywords is required")
		}
		s, err := dbManager.CreateScenario(models.TenantScenario{
			TenantID: tenantID,
			Keywords: keywords,
			Response: response,
			Status:   "draft",
		})
		if err != nil {
			return mcpErr("failed to create scenario: " + err.Error())
		}
		return mcpText(fmt.Sprintf("Draft scenario created.\nID: %s\nKeywords: %s\nResponse: %s\n\nApprove in dashboard to activate.",
			s.ID, strings.Join(s.Keywords, ", "), s.Response))

	case "list_scenarios":
		status := strArg("status")
		var scenarios []models.TenantScenario
		var err error
		switch status {
		case "active":
			scenarios, err = dbManager.GetActiveScenariosForTenant(tenantID)
		case "draft":
			scenarios, err = dbManager.GetDraftScenariosForTenant(tenantID)
		default:
			scenarios, err = dbManager.GetScenariosForTenant(tenantID)
		}
		if err != nil {
			return mcpErr("failed to list scenarios: " + err.Error())
		}
		if len(scenarios) == 0 {
			return mcpText("No scenarios found.")
		}
		var sb strings.Builder
		for _, sc := range scenarios {
			fmt.Fprintf(&sb, "[%s] id=%s keywords=%s\n  response: %s\n",
				sc.Status, sc.ID, strings.Join(sc.Keywords, ", "), sc.Response)
		}
		return mcpText(sb.String())

	case "get_tenant":
		name := dbManager.TenantName(tenantID)
		active, _ := dbManager.GetActiveScenariosForTenant(tenantID)
		drafts, _ := dbManager.GetDraftScenariosForTenant(tenantID)
		return mcpText(fmt.Sprintf("Tenant: %s (id: %s)\nActive scenarios: %d\nDraft scenarios: %d",
			name, tenantID, len(active), len(drafts)))

	default:
		return mcpErr("unknown management tool: " + tool)
	}
}

// HandleMCPMessage handles POST /mcp/message.
// Supports both plain JSON and SSE responses (streamable HTTP transport, spec 2025-03-26).
// When the client sends Accept: text/event-stream the JSON-RPC response is wrapped as an SSE event.
func HandleMCPMessage(dbManager *db.TenantDBManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := r.Context().Value(TenantIDKey).(string)
		if denyPrivateTenant(w, r, dbManager, tenantID) {
			return
		}
		log.Printf("[MCP] POST /mcp/message tenant=%s\n", tenantID)

		var req models.JSONRPCRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid JSON-RPC", http.StatusBadRequest)
			return
		}

		resp := models.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID}
		tenantScenarios, _ := dbManager.GetActiveScenariosForTenant(tenantID)

		switch req.Method {
		case "tools/call":
			var params struct {
				Name      string          `json:"name"`
				Arguments json.RawMessage `json:"arguments"`
			}
			if err := json.Unmarshal(req.Params, &params); err != nil {
				resp.Error = map[string]interface{}{"code": -32602, "message": "Invalid params"}
				break
			}
			mgmtToolNames := map[string]bool{"create_scenario": true, "list_scenarios": true, "get_tenant": true}
			if mgmtToolNames[params.Name] {
				if !isOwner(r, dbManager, tenantID) {
					resp.Error = map[string]interface{}{"code": -32603, "message": "unauthorized: management tools require tenant ownership"}
					break
				}
				resp.Result = handleMgmtTool(dbManager, tenantID, params.Name, params.Arguments)
			} else {
				scenario := registry.MatchTool(params.Name, tenantScenarios)
				toolData := scenario.MCPToolData
				resp.Result = map[string]interface{}{
					"content": []map[string]interface{}{
						{"type": "text", "text": "Successfully retrieved mock dataset."},
						{"type": "json", "data": toolData},
					},
				}
			}

		case "tools/list":
			var tools []interface{}
			for _, t := range registry.ListTools(tenantScenarios) {
				tools = append(tools, t)
			}
			if isOwner(r, dbManager, tenantID) {
				for _, t := range mgmtTools {
					tools = append(tools, t)
				}
			}
			resp.Result = map[string]interface{}{"tools": tools}

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
		if denyPrivateTenant(w, r, dbManager, tenantID) {
			return
		}

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
