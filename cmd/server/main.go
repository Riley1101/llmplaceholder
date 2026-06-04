package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	adapter "llmplaceholder/internal/adapters/http"
	"llmplaceholder/internal/adapters/stdio"
	"llmplaceholder/internal/chaos"
	"llmplaceholder/internal/db"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "stdio" {
		// You would need a stdio adapter (like the one we discussed earlier)
		stdio.Serve()
		return
	}
	chaosManager := chaos.NewManager()

	dbManager, err := db.NewTenantDBManager("./data/tenants.db")
	if err != nil {
		log.Fatalf("Failed to init DB: %v", err)
	}
	dbManager.MigrateFromFiles("./data/tenants")

	if err := adapter.LoadTemplates("./frontend/templates"); err != nil {
		log.Fatalf("Failed to load templates: %v", err)
	}

	mux := http.NewServeMux()

	// Pages + static assets
	mux.HandleFunc("/", adapter.HandleIndex())
	mux.HandleFunc("/routes", adapter.HandleRoutes())
	mux.HandleFunc("/playground", adapter.HandlePlayground())
	mux.Handle("/assets/", http.FileServer(http.Dir("./frontend")))

	// ── LLM / MCP ────────────────────────────────────────────────────────────
	mux.HandleFunc("POST /v1/chat/completions",
		adapter.TenantMiddleware(chaosManager.Middleware(adapter.HandleOpenAI(dbManager))))
	mux.HandleFunc("POST /mcp/message",
		adapter.TenantMiddleware(adapter.HandleMCPMessage(dbManager)))
	mux.HandleFunc("GET /mcp/sse",
		adapter.TenantMiddleware(adapter.HandleMCPSSE(dbManager)))
	mux.HandleFunc("GET /ui/tenants", adapter.HandleTenantSidebar(dbManager))
	mux.HandleFunc("GET /ui/tenants/{id}", adapter.HandleTenantPanel(dbManager))
	mux.HandleFunc("GET /ui/tenants/{id}/overview", adapter.HandleTenantOverview(dbManager))
	mux.HandleFunc("GET /ui/tenants/{id}/mcp", adapter.HandleTenantMCP(dbManager))

	// ── Admin — tenant CRUD ───────────────────────────────────────────────────
	mux.HandleFunc("GET /admin/tenants", adapter.HandleListTenants(dbManager))
	mux.HandleFunc("POST /admin/tenants", adapter.HandleCreateTenant(dbManager))
	mux.HandleFunc("GET /admin/tenants/{id}", adapter.HandleGetTenant(dbManager))
	mux.HandleFunc("PUT /admin/tenants/{id}/state", adapter.HandleUpdateTenantState(dbManager))
	mux.HandleFunc("DELETE /admin/tenants/{id}", adapter.HandleDeleteTenant(dbManager))

	// ── Admin — scenario CRUD ─────────────────────────────────────────────────
	mux.HandleFunc("GET /admin/tenants/{id}/scenarios", adapter.HandleListScenarios(dbManager))
	mux.HandleFunc("POST /admin/tenants/{id}/scenarios", adapter.HandleCreateScenario(dbManager))
	mux.HandleFunc("DELETE /admin/tenants/{id}/scenarios/{sid}", adapter.HandleDeleteScenario(dbManager))

	// ── Admin — settings ─────────────────────────────────────────────────────
	mux.HandleFunc("GET /admin/tenants/{id}/settings", adapter.HandleGetTenantSettings(dbManager))
	mux.HandleFunc("PATCH /admin/tenants/{id}/settings", adapter.HandlePatchTenantSettings(dbManager))

	// ── Admin — chaos ─────────────────────────────────────────────────────────
	mux.HandleFunc("GET /admin/tenants/{id}/chaos", adapter.HandleGetTenantChaos(chaosManager))
	mux.HandleFunc("POST /admin/tenants/{id}/chaos", adapter.HandleSetTenantChaos(chaosManager))
	mux.HandleFunc("POST /admin/chaos", adapter.TenantMiddleware(adapter.HandleSetChaos(chaosManager)))
	mux.HandleFunc("POST /admin/reset", adapter.TenantMiddleware(adapter.HandleResetTenant(dbManager)))

	port := ":8080"
	fmt.Printf("🚀 llmplaceholder server running on http://localhost%s\n", port)

	if err := http.ListenAndServe(port, adapter.CORSMiddleware(mux)); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
