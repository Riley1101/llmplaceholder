package http

import (
	"fmt"
	"html"
	"net/http"
	"net/url"

	"llmplaceholder/internal/db"
)

func HandleTenantSidebar(dbManager *db.TenantDBManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenants, err := dbManager.ListTenants()
		if err != nil {
			http.Error(w, "failed to load tenants", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if len(tenants) == 0 {
			_, _ = fmt.Fprint(w, `<div class="empty-state"><h3>No tenants yet</h3><p>Create a tenant through the admin API and this panel will refresh automatically.</p></div>`)
			return
		}

		_, _ = fmt.Fprintf(w, `<div class="tenant-count">%d tenants</div><div class="tenant-list">`, len(tenants))
		for _, tenantID := range tenants {
			escapedTenantID := html.EscapeString(tenantID)
			pathTenantID := url.PathEscape(tenantID)
			_, _ = fmt.Fprintf(w, `<button type="button" class="tenant-chip" hx-get="/ui/tenants/%s/overview" hx-target="#tenant-detail" hx-swap="innerHTML"><span class="dot"></span><span>%s</span></button>`, pathTenantID, escapedTenantID)
		}
		_, _ = fmt.Fprint(w, `</div>`)
	}
}

func HandleTenantPanel(dbManager *db.TenantDBManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := r.PathValue("id")
		_, _ = dbManager.ReadState(tenantID)
		renderTenantPanel(w, tenantID, "overview", renderTenantOverview(tenantID))
	}
}

func HandleTenantOverview(dbManager *db.TenantDBManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := r.PathValue("id")
		_, _ = dbManager.ReadState(tenantID)
		renderTenantPanel(w, tenantID, "overview", renderTenantOverview(tenantID))
	}
}

func HandleTenantMCP(dbManager *db.TenantDBManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := r.PathValue("id")
		_, _ = dbManager.ReadState(tenantID)
		renderTenantPanel(w, tenantID, "mcp", renderTenantMCP(tenantID))
	}
}

func renderTenantPanel(w http.ResponseWriter, tenantID, activeTab, content string) {
	pathTenantID := url.PathEscape(tenantID)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = fmt.Fprintf(w, `
<section class="tenant-panel card">
	<div class="card-head tenant-panel-head">
		<div>
			<p class="eyebrow">Selected tenant</p>
			<h2>%s</h2>
		</div>
		<div class="tenant-pill">Active</div>
	</div>
	<div class="tenant-tabs">
		<button type="button" class="tenant-tab %s" hx-get="/ui/tenants/%s/overview" hx-target="#tenant-detail" hx-swap="innerHTML">Overview</button>
		<button type="button" class="tenant-tab %s" hx-get="/ui/tenants/%s/mcp" hx-target="#tenant-detail" hx-swap="innerHTML">MCP</button>
	</div>
	<div class="tenant-panel-body">%s</div>
</section>`, html.EscapeString(tenantID), activeClass(activeTab, "overview"), pathTenantID, activeClass(activeTab, "mcp"), pathTenantID, content)
}

func renderTenantOverview(tenantID string) string {
	return fmt.Sprintf(`
<div class="detail-grid">
	<div class="detail-card">
		<p class="eyebrow">Tenant state</p>
		<p class="detail-value">%s</p>
		<p class="detail-note">This tenant is provisioned through the backend SQLite store and can be used for admin and MCP flows.</p>
	</div>
	<div class="detail-card">
		<p class="eyebrow">Quick links</p>
		<ul class="detail-list">
			<li><span>Admin state</span><em>/admin/tenants/%s</em></li>
			<li><span>Chaos profile</span><em>/admin/tenants/%s/chaos</em></li>
			<li><span>MCP transport</span><em>header-based</em></li>
		</ul>
	</div>
</div>`, html.EscapeString(tenantID), url.PathEscape(tenantID), url.PathEscape(tenantID))
}

func renderTenantMCP(tenantID string) string {
	return fmt.Sprintf(`
<div class="detail-grid">
	<div class="detail-card">
		<p class="eyebrow">MCP tab</p>
		<h3>Tenant-scoped JSON-RPC</h3>
		<p class="detail-note">This tab keeps the MCP flow attached to %s instead of exposing it as a global top-level panel.</p>
	</div>
	<div class="detail-card">
		<p class="eyebrow">Request</p>
		<pre class="code-block">POST /mcp/message
X-Tenant-ID: %s
Content-Type: application/json

{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}</pre>
	</div>
</div>`, html.EscapeString(tenantID), html.EscapeString(tenantID))
}

func activeClass(activeTab, tabName string) string {
	if activeTab == tabName {
		return "active"
	}
	return ""
}
