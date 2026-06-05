package mcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"llmplaceholder/internal/db"
	"llmplaceholder/internal/core/models"
)

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *rpcError   `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func textContent(s string) interface{} {
	return map[string]interface{}{
		"content": []map[string]string{{"type": "text", "text": s}},
	}
}

func errContent(s string) interface{} {
	return map[string]interface{}{
		"content":  []map[string]string{{"type": "text", "text": s}},
		"isError":  true,
	}
}

func send(enc *json.Encoder, resp rpcResponse) {
	if err := enc.Encode(resp); err != nil {
		log.Println("[MCP] encode error:", err)
	}
}

// Serve starts the MCP stdio server. It reads LLMP_DB_PATH, LLMP_TENANT_ID,
// and LLMP_MCP_KEY from the environment to connect and authenticate.
func Serve() {
	dbPath := os.Getenv("LLMP_DB_PATH")
	if dbPath == "" {
		dbPath = "./data/tenants.db"
	}
	tenantID := os.Getenv("LLMP_TENANT_ID")
	mcpKey := os.Getenv("LLMP_MCP_KEY")

	dbManager, err := db.NewTenantDBManager(dbPath)
	if err != nil {
		log.Fatalf("[MCP] DB init failed: %v", err)
	}

	scanner := bufio.NewScanner(os.Stdin)
	enc := json.NewEncoder(os.Stdout)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var req rpcRequest
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			continue
		}

		// Notifications have no ID — no response required.
		if req.ID == nil && strings.HasPrefix(req.Method, "notifications/") {
			continue
		}

		resp := rpcResponse{JSONRPC: "2.0", ID: req.ID}

		switch req.Method {
		case "initialize":
			resp.Result = map[string]interface{}{
				"protocolVersion": "2024-11-05",
				"capabilities":    map[string]interface{}{"tools": map[string]interface{}{}},
				"serverInfo":      map[string]string{"name": "llmplaceholder", "version": "1.0.0"},
			}

		case "tools/list":
			resp.Result = map[string]interface{}{"tools": toolDefs()}

		case "tools/call":
			var p struct {
				Name      string                 `json:"name"`
				Arguments map[string]interface{} `json:"arguments"`
			}
			if err := json.Unmarshal(req.Params, &p); err != nil {
				resp.Error = &rpcError{Code: -32602, Message: "invalid params"}
				send(enc, resp)
				continue
			}
			resp.Result = dispatch(dbManager, tenantID, mcpKey, p.Name, p.Arguments)

		case "ping":
			resp.Result = map[string]interface{}{}

		default:
			resp.Error = &rpcError{Code: -32601, Message: "method not found: " + req.Method}
		}

		send(enc, resp)
	}
}

// dispatch routes tool calls after validating auth.
func dispatch(dbm *db.TenantDBManager, envTenantID, envKey, tool string, args map[string]interface{}) interface{} {
	// Allow per-call override of tenant_id / api_key (fallback to env).
	tenantID := stringArg(args, "tenant_id")
	if tenantID == "" {
		tenantID = envTenantID
	}
	apiKey := stringArg(args, "api_key")
	if apiKey == "" {
		apiKey = envKey
	}

	if tenantID == "" {
		return errContent("tenant_id is required (set LLMP_TENANT_ID env or pass tenant_id arg)")
	}
	if !dbm.ValidateMCPKey(tenantID, apiKey) {
		return errContent("invalid api_key for tenant " + tenantID)
	}

	switch tool {
	case "create_scenario":
		return toolCreateScenario(dbm, tenantID, args)
	case "list_scenarios":
		return toolListScenarios(dbm, tenantID, args)
	case "get_tenant":
		return toolGetTenant(dbm, tenantID)
	default:
		return errContent("unknown tool: " + tool)
	}
}

func toolCreateScenario(dbm *db.TenantDBManager, tenantID string, args map[string]interface{}) interface{} {
	response := stringArg(args, "response")
	if response == "" {
		return errContent("response is required")
	}

	// keywords can be a string (comma-separated) or []interface{}
	var keywords []string
	switch v := args["keywords"].(type) {
	case string:
		for _, k := range strings.Split(v, ",") {
			k = strings.TrimSpace(k)
			if k != "" {
				keywords = append(keywords, k)
			}
		}
	case []interface{}:
		for _, item := range v {
			if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
				keywords = append(keywords, strings.TrimSpace(s))
			}
		}
	}
	if len(keywords) == 0 {
		return errContent("keywords is required (comma-separated string or array)")
	}

	s, err := dbm.CreateScenario(models.TenantScenario{
		TenantID: tenantID,
		Keywords: keywords,
		Response: response,
		Status:   "draft",
	})
	if err != nil {
		return errContent("failed to create scenario: " + err.Error())
	}

	msg := fmt.Sprintf(
		"Draft scenario created.\nID: %s\nKeywords: %s\nResponse: %s\n\nReview it in the dashboard at /playground (Scenarios tab) and approve to activate.",
		s.ID, strings.Join(s.Keywords, ", "), s.Response,
	)
	return textContent(msg)
}

func toolListScenarios(dbm *db.TenantDBManager, tenantID string, args map[string]interface{}) interface{} {
	status := stringArg(args, "status") // "active", "draft", or "" (all)

	var scenarios []models.TenantScenario
	var err error
	switch status {
	case "active":
		scenarios, err = dbm.GetActiveScenariosForTenant(tenantID)
	case "draft":
		scenarios, err = dbm.GetDraftScenariosForTenant(tenantID)
	default:
		scenarios, err = dbm.GetScenariosForTenant(tenantID)
	}
	if err != nil {
		return errContent("failed to list scenarios: " + err.Error())
	}

	if len(scenarios) == 0 {
		return textContent("No scenarios found.")
	}

	var sb strings.Builder
	for _, s := range scenarios {
		fmt.Fprintf(&sb, "[%s] id=%s keywords=%s\n  response: %s\n",
			s.Status, s.ID, strings.Join(s.Keywords, ", "), s.Response)
	}
	return textContent(sb.String())
}

func toolGetTenant(dbm *db.TenantDBManager, tenantID string) interface{} {
	exists, err := dbm.TenantExists(tenantID)
	if err != nil || !exists {
		return errContent("tenant not found: " + tenantID)
	}
	name := dbm.TenantName(tenantID)
	active, _ := dbm.GetActiveScenariosForTenant(tenantID)
	drafts, _ := dbm.GetDraftScenariosForTenant(tenantID)
	msg := fmt.Sprintf("Tenant: %s (id: %s)\nActive scenarios: %d\nDraft scenarios: %d",
		name, tenantID, len(active), len(drafts))
	return textContent(msg)
}

func toolDefs() []map[string]interface{} {
	return []map[string]interface{}{
		{
			"name":        "create_scenario",
			"description": "Create a draft scenario for the tenant. It will appear in the dashboard for human review before going active.",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"keywords": map[string]string{
						"type":        "string",
						"description": "Comma-separated keywords that trigger this scenario (e.g. 'invoice, billing, payment')",
					},
					"response": map[string]string{
						"type":        "string",
						"description": "The mock response text to stream back when a prompt matches these keywords",
					},
					"tenant_id": map[string]string{
						"type":        "string",
						"description": "Tenant ID (overrides LLMP_TENANT_ID env)",
					},
					"api_key": map[string]string{
						"type":        "string",
						"description": "MCP API key for the tenant (overrides LLMP_MCP_KEY env)",
					},
				},
				"required": []string{"keywords", "response"},
			},
		},
		{
			"name":        "list_scenarios",
			"description": "List scenarios for the tenant, optionally filtered by status.",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"status": map[string]string{
						"type":        "string",
						"description": "Filter by status: 'active', 'draft', or omit for all",
					},
					"tenant_id": map[string]string{"type": "string"},
					"api_key":   map[string]string{"type": "string"},
				},
			},
		},
		{
			"name":        "get_tenant",
			"description": "Get basic info about the tenant including scenario counts.",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"tenant_id": map[string]string{"type": "string"},
					"api_key":   map[string]string{"type": "string"},
				},
			},
		},
	}
}

func stringArg(args map[string]interface{}, key string) string {
	if v, ok := args[key]; ok {
		if s, ok := v.(string); ok {
			return strings.TrimSpace(s)
		}
	}
	return ""
}
