package registry

import "llmplaceholder/internal/core/models"

// GlobalRegistry organizes mock responses by scenario keywords
var GlobalRegistry = []models.MockScenario{
	{
		ID:           "fintech_billing",
		Keywords:     []string{"invoice", "billing", "charge"},
		FullResponse: "I have checked the accounting ledger. The highest pending amount is Invoice **#INV-2026-0892** for Acme Corp ($14,250.00). Would you like me to trigger a follow-up email?",
		MCPToolName:  "get_invoice_ledger",
		MCPToolData: map[string]interface{}{
			"status": "success",
			"data": []map[string]interface{}{
				{"id": "INV-2026-0892", "client": "Acme Corp", "amount": 14250.00, "status": "OVERDUE"},
			},
		},
	},
	{
		ID:           "sre_cluster_degradation",
		Keywords:     []string{"latency", "memory", "crash", "kubernetes"},
		FullResponse: "An analysis of the core-db-replica-01 clusters shows a memory spike to 98.2% starting at 14:22 UTC. I recommend isolating the connection pool.",
		MCPToolName:  "fetch_pod_metrics",
		MCPToolData: map[string]interface{}{
			"cluster": "core-db-replica-01",
			"metrics": map[string]interface{}{
				"ram_utilization":    "98.2%",
				"active_connections": 1420,
			},
		},
	},
	{
		ID:           "fallback",
		Keywords:     []string{},
		FullResponse: "I received your prompt, but no specific demo keywords were matched. I am running in local mock mode.",
		MCPToolName:  "unknown",
		MCPToolData:  map[string]string{"error": "Tool not found in mock registry."},
	},
}
