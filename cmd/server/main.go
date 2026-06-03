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
	dbManager := db.NewTenantDBManager("./data/tenants", "./data/template.json")

	mux := http.NewServeMux()

	mux.HandleFunc("POST /v1/chat/completions", adapter.TenantMiddleware(chaosManager.Middleware(adapter.HandleOpenAI)))
	mux.HandleFunc("POST /mcp/message", adapter.TenantMiddleware(adapter.HandleMCPMessage(dbManager)))
	mux.HandleFunc("POST /admin/reset", adapter.TenantMiddleware(adapter.HandleResetTenant(dbManager)))

	port := ":8080"
	fmt.Printf("🚀 llmplaceholder server running on http://localhost%s\n", port)

	if err := http.ListenAndServe(port, mux); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
