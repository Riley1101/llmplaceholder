package registry

import (
	"strings"

	"llmplaceholder/internal/core/models"
)

const fallbackResponse = "No matching scenario found for this tenant. Add scenarios in the tenant dashboard."

// MatchIntent finds the first tenant scenario whose keywords match the prompt.
func MatchIntent(prompt string, tenantScenarios []models.TenantScenario) models.MockScenario {
	cleaned := strings.ToLower(prompt)
	for _, s := range tenantScenarios {
		for _, kw := range s.Keywords {
			if strings.Contains(cleaned, strings.ToLower(kw)) {
				return toMockScenario(s)
			}
		}
	}
	return models.MockScenario{
		FullResponse: fallbackResponse,
		MCPToolName:  "unknown",
		MCPToolData:  map[string]string{"error": "no matching scenario"},
	}
}

// MatchTool finds the first tenant scenario with a matching tool name.
func MatchTool(toolName string, tenantScenarios []models.TenantScenario) models.MockScenario {
	for _, s := range tenantScenarios {
		if s.ToolName == toolName {
			return toMockScenario(s)
		}
	}
	return models.MockScenario{
		MCPToolName: toolName,
		MCPToolData: map[string]string{"error": "tool not found in tenant scenario registry"},
	}
}

// ListTools returns one entry per unique MCP tool defined in tenant scenarios.
func ListTools(tenantScenarios []models.TenantScenario) []map[string]string {
	seen := map[string]bool{}
	var tools []map[string]string
	for _, s := range tenantScenarios {
		if s.ToolName == "" || seen[s.ToolName] {
			continue
		}
		seen[s.ToolName] = true
		tools = append(tools, map[string]string{"name": s.ToolName})
	}
	return tools
}

func toMockScenario(s models.TenantScenario) models.MockScenario {
	return models.MockScenario{
		ID:           s.ID,
		Keywords:     s.Keywords,
		FullResponse: s.Response,
		MCPToolName:  s.ToolName,
		MCPToolData:  s.ToolData,
	}
}
