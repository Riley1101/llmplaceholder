package models

import (
	"encoding/json"
	"time"
)

// Protocol identifies the origin signature of the incoming client
type Protocol string

// TenantContextKey is a typed key to prevent context collisions
type TenantContextKey string

const TenantIDKey TenantContextKey = "tenant_id"

const (
	ProtocolOpenAI    Protocol = "openai"
	ProtocolAnthropic Protocol = "anthropic"
	ProtocolMCP       Protocol = "mcp"
	ProtocolA2A       Protocol = "a2a"
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

// TenantScenario is a user-defined scenario stored per-tenant in SQLite
type TenantScenario struct {
	ID       string      `json:"id"`
	TenantID string      `json:"tenant_id"`
	Keywords []string    `json:"keywords"`
	Response string      `json:"response"`
	ToolName string      `json:"tool_name,omitempty"`
	ToolData interface{} `json:"tool_data,omitempty"`
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

// AnthropicMessageRequest represents standard /v1/messages payload
type AnthropicMessageRequest struct {
	Model     string `json:"model"`
	MaxTokens int    `json:"max_tokens"`
	Messages  []struct {
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

// TenantMeta carries tenant identity plus ownership flag for UI rendering.
type TenantMeta struct {
	ID       string `json:"id"`
	IsGlobal bool   `json:"is_global"`
}

// User is an authenticated user identified via GitHub OAuth.
type User struct {
	ID        string
	GithubID  int64
	Login     string
	Name      string
	Email     string
	AvatarURL string
}

// APIToken represents a user-generated bearer token for programmatic API access.
type APIToken struct {
	ID          string
	Name        string
	CreatedAt   time.Time
	LastUsedAt  *time.Time
}
