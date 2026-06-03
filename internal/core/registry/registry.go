package registry

import (
	"strings"

	"llmplaceholder/internal/core/models"
)

// MatchIntent scans the user prompt and returns the closest high-fidelity scenario
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

// MatchTool searches the registry for an exact MCP tool target
func MatchTool(toolName string) models.MockScenario {
	for _, scenario := range GlobalRegistry {
		if scenario.MCPToolName == toolName {
			return scenario
		}
	}

	return getFallback()
}

// getFallback safely returns the default scenario
func getFallback() models.MockScenario {
	for _, scenario := range GlobalRegistry {
		if scenario.ID == "fallback" {
			return scenario
		}
	}
	return models.MockScenario{}
}
