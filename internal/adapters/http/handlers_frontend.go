package http

import (
	"net/http"
	"net/url"

	"llmplaceholder/internal/core/models"
	"llmplaceholder/internal/db"
)

type pageData struct {
	User *models.User
}

type sidebarData struct {
	Tenants []models.TenantMeta
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
		render(w, "index", pageData{User: UserFromContext(r)})
	}
}

func HandleRoutes() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		render(w, "routes", pageData{User: UserFromContext(r)})
	}
}

func HandlePlayground() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		render(w, "playground", pageData{User: UserFromContext(r)})
	}
}

func HandleDocs() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		render(w, "docs", pageData{User: UserFromContext(r)})
	}
}

func HandleTenantSidebar(dbManager *db.TenantDBManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := UserFromContext(r)
		if user == nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		tenants, err := dbManager.ListTenantsForUser(user.ID)
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
