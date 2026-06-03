package models

import "encoding/json"

// Protocol identifies the origin signature of the incoming client
type Protocol string

// TenantContextKey is a typed key to prevent context collisions
type TenantContextKey string

const TenantIDKey TenantContextKey = "tenant_id"

const (
	ProtocolOpenAI Protocol = "openai"
	ProtocolMCP    Protocol = "mcp"
	ProtocolA2A    Protocol = "a2a"
)

// MockScenario defines a complete conversational and functional blueprint
type MockScenario struct {
	ID           string
	Keywords     []string
	FullResponse string
	MCPToolName  string
	MCPToolData  interface{}
	StateKey     string // root key to extract from tenant DB; empty means no tenant override
}

// OpenAIChatRequest represents standard /v1/chat/completions payload
type OpenAIChatRequest struct {
	Model    string `json:"model"`
	Messages []struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"messages"`
	Stream bool `json:"stream"`
}

// JSONRPCRequest represents an MCP /mcp/message payload
type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// JSONRPCResponse is the standard MCP response wrapper
type JSONRPCResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   interface{} `json:"error,omitempty"`
}
