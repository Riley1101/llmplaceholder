package main

import (
	"fmt"
	"log"
	"net/http"

	adapter "llmplaceholder/internal/adapters/http"
	"llmplaceholder/internal/chaos"
	"llmplaceholder/internal/db"
)

func main() {
	chaosManager := chaos.NewManager()

	dbManager, err := db.NewTenantDBManager("./data/tenants.db")
	if err != nil {
		log.Fatalf("Failed to init DB: %v", err)
	}
	dbManager.MigrateFromFiles("./data/tenants")

	mux := http.NewServeMux()

	// Dashboard + static assets
	mux.Handle("/", http.FileServer(http.Dir("./public")))

	// ── LLM / MCP ────────────────────────────────────────────────────────────
	mux.HandleFunc("POST /v1/chat/completions",
		adapter.TenantMiddleware(chaosManager.Middleware(adapter.HandleOpenAI(dbManager))))
	mux.HandleFunc("POST /mcp/message",
		adapter.TenantMiddleware(adapter.HandleMCPMessage(dbManager)))

	// ── Admin — tenant CRUD ───────────────────────────────────────────────────
	mux.HandleFunc("GET /admin/tenants",              adapter.HandleListTenants(dbManager))
	mux.HandleFunc("POST /admin/tenants",             adapter.HandleCreateTenant(dbManager))
	mux.HandleFunc("GET /admin/tenants/{id}",         adapter.HandleGetTenant(dbManager))
	mux.HandleFunc("PUT /admin/tenants/{id}/state",   adapter.HandleUpdateTenantState(dbManager))
	mux.HandleFunc("DELETE /admin/tenants/{id}",      adapter.HandleDeleteTenant(dbManager))

	// ── Admin — scenario CRUD ─────────────────────────────────────────────────
	mux.HandleFunc("GET /admin/tenants/{id}/scenarios",          adapter.HandleListScenarios(dbManager))
	mux.HandleFunc("POST /admin/tenants/{id}/scenarios",         adapter.HandleCreateScenario(dbManager))
	mux.HandleFunc("DELETE /admin/tenants/{id}/scenarios/{sid}", adapter.HandleDeleteScenario(dbManager))

	// ── Admin — chaos ─────────────────────────────────────────────────────────
	mux.HandleFunc("GET /admin/tenants/{id}/chaos",  adapter.HandleGetTenantChaos(chaosManager))
	mux.HandleFunc("POST /admin/tenants/{id}/chaos", adapter.HandleSetTenantChaos(chaosManager))
	mux.HandleFunc("POST /admin/chaos",              adapter.TenantMiddleware(adapter.HandleSetChaos(chaosManager)))
	mux.HandleFunc("POST /admin/reset",              adapter.TenantMiddleware(adapter.HandleResetTenant(dbManager)))

	port := ":8080"
	fmt.Printf("🚀 llmplaceholder server running on http://localhost%s\n", port)

	if err := http.ListenAndServe(port, mux); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
