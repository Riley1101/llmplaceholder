package registry

import (
	"strings"

	"llmplaceholder/internal/core/models"
)

// MatchIntent scans the user prompt against the global registry
func MatchIntent(userPrompt string) models.MockScenario {
	cleaned := strings.ToLower(userPrompt)

	for _, scenario := range GlobalRegistry {
		for _, kw := range scenario.Keywords {
			if strings.Contains(cleaned, kw) {
				return scenario
			}
		}
	}

	return getFallback()
}

// MatchIntentForTenant checks tenant-specific scenarios first, then falls back to global registry
func MatchIntentForTenant(prompt string, tenantScenarios []models.TenantScenario) models.MockScenario {
	cleaned := strings.ToLower(prompt)
	for _, s := range tenantScenarios {
		for _, kw := range s.Keywords {
			if strings.Contains(cleaned, strings.ToLower(kw)) {
				return models.MockScenario{
					ID:           s.ID,
					Keywords:     s.Keywords,
					FullResponse: s.Response,
					MCPToolName:  s.ToolName,
					MCPToolData:  s.ToolData,
				}
			}
		}
	}
	return MatchIntent(prompt)
}

// MatchTool searches the global registry for an exact MCP tool target
func MatchTool(toolName string) models.MockScenario {
	for _, scenario := range GlobalRegistry {
		if scenario.MCPToolName == toolName {
			return scenario
		}
	}

	return getFallback()
}

// MatchToolForTenant checks tenant-specific scenarios first, then falls back to global registry
func MatchToolForTenant(toolName string, tenantScenarios []models.TenantScenario) models.MockScenario {
	for _, s := range tenantScenarios {
		if s.ToolName == toolName {
			return models.MockScenario{
				ID:           s.ID,
				Keywords:     s.Keywords,
				FullResponse: s.Response,
				MCPToolName:  s.ToolName,
				MCPToolData:  s.ToolData,
			}
		}
	}
	return MatchTool(toolName)
}

// ListTools returns one entry per unique non-fallback MCP tool in the global registry
func ListTools() []map[string]string {
	seen := map[string]bool{}
	var tools []map[string]string
	for _, s := range GlobalRegistry {
		if s.MCPToolName == "" || s.MCPToolName == "unknown" || seen[s.MCPToolName] {
			continue
		}
		seen[s.MCPToolName] = true
		tools = append(tools, map[string]string{"name": s.MCPToolName})
	}
	return tools
}

// ListToolsForTenant merges tenant-specific tools with global tools.
// Set includeGlobal=false to return only tenant-defined tools.
func ListToolsForTenant(tenantScenarios []models.TenantScenario, includeGlobal bool) []map[string]string {
	seen := map[string]bool{}
	var tools []map[string]string
	for _, s := range tenantScenarios {
		if s.ToolName == "" || seen[s.ToolName] {
			continue
		}
		seen[s.ToolName] = true
		tools = append(tools, map[string]string{"name": s.ToolName, "source": "tenant"})
	}
	if includeGlobal {
		for _, t := range ListTools() {
			if !seen[t["name"]] {
				t["source"] = "global"
				tools = append(tools, t)
			}
		}
	}
	return tools
}

func getFallback() models.MockScenario {
	for _, scenario := range GlobalRegistry {
		if scenario.ID == "fallback" {
			return scenario
		}
	}
	return models.MockScenario{}
}
