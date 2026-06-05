package stdio

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"llmplaceholder/internal/core/models"
	"llmplaceholder/internal/core/registry"
)

// Serve starts the JSON-RPC loop over standard I/O.
func Serve() {
	// 🚨 CRITICAL: When using STDIO transport, you CANNOT log to stdout.
	// Any non-JSON text printed to stdout will crash the client's parser.
	// All logging must be explicitly routed to stderr.
	logger := log.New(os.Stderr, "[MCP STDIO] ", log.LstdFlags)
	logger.Println("Starting STDIO server. Waiting for JSON-RPC messages on Stdin...")

	// STDIO doesn't have HTTP headers, so we assign a static tenant for local dev
	tenantID := "local_mcp_client"

	// Read line-by-line from Stdin
	scanner := bufio.NewScanner(os.Stdin)

	for scanner.Scan() {
		line := scanner.Bytes()
		logger.Printf("Received payload size: %d bytes", len(line))

		var req models.JSONRPCRequest
		if err := json.Unmarshal(line, &req); err != nil {
			sendError(nil, -32700, "Parse error")
			continue
		}

		resp := models.JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
		}

		// Dispatch the JSON-RPC Method
		switch req.Method {
		case "tools/call":
			var params struct {
				Name string `json:"name"`
			}
			if err := json.Unmarshal(req.Params, &params); err != nil {
				resp.Error = map[string]interface{}{"code": -32602, "message": "Invalid params"}
				break
			}

			logger.Printf("Executing tool '%s' for tenant '%s'", params.Name, tenantID)
			scenario := registry.MatchTool(params.Name, nil)

			resp.Result = map[string]interface{}{
				"content": []map[string]interface{}{
					{
						"type": "text",
						"text": "Successfully retrieved mock dataset via STDIO.",
					},
					{
						"type": "json",
						"data": scenario.MCPToolData,
					},
				},
			}

		case "tools/list":
			logger.Println("Listing available tools...")
			resp.Result = map[string]interface{}{
				"tools": registry.ListTools(nil),
			}

		default:
			resp.Error = map[string]interface{}{"code": -32601, "message": "Method not found"}
		}

		// Write the formatted JSON-RPC response back to Stdout
		out, _ := json.Marshal(resp)
		fmt.Println(string(out))
	}

	if err := scanner.Err(); err != nil {
		logger.Fatalf("Scanner error: %v", err)
	}
}

// sendError formats and outputs a standard JSON-RPC error
func sendError(id interface{}, code int, message string) {
	resp := models.JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error:   map[string]interface{}{"code": code, "message": message},
	}
	out, _ := json.Marshal(resp)
	fmt.Println(string(out))
}
