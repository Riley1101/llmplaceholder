package http

import (
	"net/http"
	"net/url"

	"llmplaceholder/internal/db"
)

type sidebarData struct {
	Tenants []string
}

type panelData struct {
	TenantID  string
	PathID    string
	ActiveTab string
}

func newPanelData(tenantID, activeTab string) panelData {
	return panelData{
		TenantID:  tenantID,
		PathID:    url.PathEscape(tenantID),
		ActiveTab: activeTab,
	}
}

func HandleIndex() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		render(w, "index", nil)
	}
}

func HandlePlayground() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		render(w, "playground", nil)
	}
}

func HandleTenantSidebar(dbManager *db.TenantDBManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenants, err := dbManager.ListTenants()
		if err != nil {
			http.Error(w, "failed to load tenants", http.StatusInternalServerError)
			return
		}
		renderPartial(w, "tenant-sidebar", sidebarData{Tenants: tenants})
	}
}

func HandleTenantPanel(dbManager *db.TenantDBManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := r.PathValue("id")
		_, _ = dbManager.ReadState(tenantID)
		renderPartial(w, "tenant-panel", newPanelData(tenantID, "overview"))
	}
}

func HandleTenantOverview(dbManager *db.TenantDBManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := r.PathValue("id")
		_, _ = dbManager.ReadState(tenantID)
		renderPartial(w, "tenant-panel", newPanelData(tenantID, "overview"))
	}
}

func HandleTenantMCP(dbManager *db.TenantDBManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := r.PathValue("id")
		_, _ = dbManager.ReadState(tenantID)
		renderPartial(w, "tenant-panel", newPanelData(tenantID, "mcp"))
	}
}
