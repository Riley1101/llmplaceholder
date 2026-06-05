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
	"github.com/joho/godotenv"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "stdio" {
		// You would need a stdio adapter (like the one we discussed earlier)
		stdio.Serve()
		return
	}

	_ = godotenv.Load()

	chaosManager := chaos.NewManager()

	dbManager, err := db.NewTenantDBManager("./data/tenants.db")
	if err != nil {
		log.Fatalf("Failed to init DB: %v", err)
	}
	dbManager.MigrateFromFiles("./data/tenants")

	if err := adapter.LoadTemplates("./frontend/templates"); err != nil {
		log.Fatalf("Failed to load templates: %v", err)
	}

	ra := adapter.RequireAuth
	mux := http.NewServeMux()

	// Pages + static assets (playground is public — auth checked per-operation)
	mux.HandleFunc("/", adapter.HandleIndex())
	mux.HandleFunc("/routes", adapter.HandleRoutes())
	mux.HandleFunc("/playground", adapter.HandlePlayground())
	mux.HandleFunc("/docs", adapter.HandleDocs())
	mux.Handle("/assets/", http.FileServer(http.Dir("./frontend")))

	// Auth
	mux.HandleFunc("GET /login", adapter.HandleLogin())
	mux.HandleFunc("GET /auth/github", adapter.HandleGithubLogin())
	mux.HandleFunc("GET /auth/github/callback", adapter.HandleGithubCallback(dbManager))
	mux.HandleFunc("POST /auth/logout", adapter.HandleLogout(dbManager))

	// ── LLM / MCP (always public) ────────────────────────────────────────────
	mux.HandleFunc("POST /v1/chat/completions",
		adapter.TenantMiddleware(chaosManager.Middleware(adapter.HandleOpenAI(dbManager))))
	mux.HandleFunc("POST /v1/messages",
		adapter.TenantMiddleware(chaosManager.Middleware(adapter.HandleAnthropic(dbManager))))
	mux.HandleFunc("POST /mcp/message",
		adapter.StrictTenantMiddleware(adapter.HandleMCPMessage(dbManager)))
	mux.HandleFunc("GET /mcp/sse",
		adapter.StrictTenantMiddleware(adapter.HandleMCPSSE(dbManager)))

	// ── UI reads (public; per-handler access control) ─────────────────────────
	mux.HandleFunc("GET /ui/tenants",                      adapter.HandleUIGetTenants(dbManager))
	mux.HandleFunc("GET /ui/tenants/{id}",                 adapter.HandleUIGetTenantDetail(dbManager))
	mux.HandleFunc("GET /ui/tenants/{id}/state",           adapter.HandleUIGetStateTab(dbManager))
	mux.HandleFunc("GET /ui/tenants/{id}/scenarios",       adapter.HandleUIGetScenariosTab(dbManager))
	mux.HandleFunc("GET /ui/tenants/{id}/tools",           adapter.HandleUIGetToolsTab(dbManager))
	mux.HandleFunc("GET /ui/tenants/{id}/chaos",           adapter.HandleUIGetChaosTab(chaosManager, dbManager))

	// ── UI writes (require login) ─────────────────────────────────────────────
	mux.HandleFunc("POST /ui/tenants",                             ra(adapter.HandleUICreateTenant(dbManager)))
	mux.HandleFunc("DELETE /ui/tenants/{id}",                      ra(adapter.HandleUIDeleteTenant(dbManager)))
	mux.HandleFunc("PUT /ui/tenants/{id}/state",                   ra(adapter.HandleUISaveState(dbManager)))
	mux.HandleFunc("POST /ui/tenants/{id}/scenarios",              ra(adapter.HandleUICreateScenario(dbManager)))
	mux.HandleFunc("DELETE /ui/tenants/{id}/scenarios/{sid}",      ra(adapter.HandleUIDeleteScenario(dbManager)))
	mux.HandleFunc("POST /ui/tenants/{id}/tools",                  ra(adapter.HandleUICreateTool(dbManager)))
	mux.HandleFunc("DELETE /ui/tenants/{id}/tools/{sid}",          ra(adapter.HandleUIDeleteTool(dbManager)))
	mux.HandleFunc("POST /ui/tenants/{id}/chaos",                  ra(adapter.HandleUISetChaos(chaosManager, dbManager)))
	mux.HandleFunc("GET /ui/api-tokens",                           ra(adapter.HandleUIGetTokens(dbManager)))
	mux.HandleFunc("POST /ui/api-tokens",                          ra(adapter.HandleUICreateToken(dbManager)))
	mux.HandleFunc("DELETE /ui/api-tokens/{id}",                   ra(adapter.HandleUIDeleteToken(dbManager)))

	// ── Admin reads (public; per-handler access control) ─────────────────────
	mux.HandleFunc("GET /admin/tenants",                        adapter.HandleListTenants(dbManager))
	mux.HandleFunc("GET /admin/tenants/{id}",                   adapter.HandleGetTenant(dbManager))
	mux.HandleFunc("GET /admin/tenants/{id}/scenarios",         adapter.HandleListScenarios(dbManager))
	mux.HandleFunc("GET /admin/tenants/{id}/settings",          adapter.HandleGetTenantSettings(dbManager))
	mux.HandleFunc("GET /admin/tenants/{id}/chaos",             adapter.HandleGetTenantChaos(chaosManager, dbManager))

	// ── Admin writes (require login) ──────────────────────────────────────────
	mux.HandleFunc("POST /admin/tenants",              ra(adapter.HandleCreateTenant(dbManager)))
	mux.HandleFunc("PUT /admin/tenants/{id}/state",    ra(adapter.HandleUpdateTenantState(dbManager)))
	mux.HandleFunc("DELETE /admin/tenants/{id}",       ra(adapter.HandleDeleteTenant(dbManager)))
	mux.HandleFunc("POST /admin/tenants/{id}/scenarios",          ra(adapter.HandleCreateScenario(dbManager)))
	mux.HandleFunc("DELETE /admin/tenants/{id}/scenarios/{sid}",  ra(adapter.HandleDeleteScenario(dbManager)))
	mux.HandleFunc("PATCH /admin/tenants/{id}/settings",          ra(adapter.HandlePatchTenantSettings(dbManager)))
	mux.HandleFunc("POST /admin/tenants/{id}/chaos",              ra(adapter.HandleSetTenantChaos(chaosManager, dbManager)))
	mux.HandleFunc("POST /admin/chaos",  ra(adapter.TenantMiddleware(adapter.HandleSetChaos(chaosManager))))
	mux.HandleFunc("POST /admin/reset",  ra(adapter.TenantMiddleware(adapter.HandleResetTenant(dbManager))))

	port := ":8080"
	fmt.Printf("🚀 llmplaceholder server running on http://localhost%s\n", port)

	if err := http.ListenAndServe(port, adapter.CORSMiddleware(adapter.AuthMiddleware(dbManager)(mux))); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
