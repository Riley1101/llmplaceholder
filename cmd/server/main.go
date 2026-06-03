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

	mux.Handle("/", http.FileServer(http.Dir("./public")))
	mux.HandleFunc("POST /v1/chat/completions", adapter.TenantMiddleware(chaosManager.Middleware(adapter.HandleOpenAI)))
	mux.HandleFunc("POST /mcp/message", adapter.TenantMiddleware(adapter.HandleMCPMessage(dbManager)))
	mux.HandleFunc("POST /admin/reset", adapter.TenantMiddleware(adapter.HandleResetTenant(dbManager)))
	mux.HandleFunc("POST /admin/chaos", adapter.TenantMiddleware(adapter.HandleSetChaos(chaosManager)))
	mux.HandleFunc("GET /admin/tenants", adapter.HandleListTenants(dbManager))

	port := ":8080"
	fmt.Printf("🚀 llmplaceholder server running on http://localhost%s\n", port)

	if err := http.ListenAndServe(port, mux); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
