package http

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"llmplaceholder/internal/chaos"
	"llmplaceholder/internal/core/models"
	"llmplaceholder/internal/db"
)

var chaosProfiles = []string{"none", "rate_limit", "server_error", "latency"}

// ── data types ────────────────────────────────────────────────────────────────

type uiDetailData struct {
	TenantID  string
	Name      string
	PathID    string
	StateJSON string
	IsGlobal  bool
}

type uiScenarioTabData struct {
	TenantID        string
	Name            string
	PathID          string
	Scenarios       []models.TenantScenario
	DraftScenarios  []models.TenantScenario
	IsGlobal        bool
}

type uiToolsTabData struct {
	TenantID    string
	Name        string
	PathID      string
	TenantTools []models.TenantScenario
	IsGlobal    bool
}

type uiChaosTabData struct {
	TenantID string
	Name     string
	PathID   string
	Profile  string
	Profiles []string
	IsGlobal bool
}

// ── helpers ───────────────────────────────────────────────────────────────────

func uiSidebar(w http.ResponseWriter, dbManager *db.TenantDBManager, userID string) {
	tenants, _ := dbManager.ListTenantsForUser(userID)
	renderPartial(w, "tenant-sidebar", sidebarData{Tenants: tenants})
}

func uiSidebarOOB(w http.ResponseWriter, dbManager *db.TenantDBManager, userID string) {
	tenants, _ := dbManager.ListTenantsForUser(userID)
	fmt.Fprint(w, `<div id="tenant-sidebar-list" hx-swap-oob="innerHTML">`)
	partials.ExecuteTemplate(w, "tenant-sidebar", sidebarData{Tenants: tenants})
	fmt.Fprint(w, `</div>`)
}

// redirectToLogin sends unauthenticated users to the login page.
// HTMX requests get HX-Redirect; others get a standard redirect.
func redirectToLogin(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", "/login")
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	http.Redirect(w, r, "/login", http.StatusFound)
}

// resolveAccess checks tenant visibility and ownership.
// Global tenants are readable by everyone. Private tenants require auth + ownership.
// Write access always requires auth; global tenants are also always read-only.
// Returns (isGlobal, ok) — writes the HTTP error itself on failure.
func resolveAccess(w http.ResponseWriter, r *http.Request, dbManager *db.TenantDBManager, tenantID string, requireWrite bool) (isGlobal bool, ok bool) {
	user := UserFromContext(r)

	ownerID, isGlobal, err := dbManager.TenantOwner(tenantID)
	if err != nil {
		http.Error(w, "tenant not found", http.StatusNotFound)
		return false, false
	}
	if isGlobal {
		if requireWrite {
			if user == nil {
				redirectToLogin(w, r)
				return false, false
			}
			http.Error(w, "global tenants are read-only", http.StatusForbidden)
			return false, false
		}
		return true, true // global tenants readable by anyone
	}
	// private tenant — always need auth
	if user == nil {
		redirectToLogin(w, r)
		return false, false
	}
	if ownerID != user.ID {
		http.Error(w, "not your tenant", http.StatusForbidden)
		return false, false
	}
	return false, true
}

func uiDetailOOBEmpty(w http.ResponseWriter) {
	fmt.Fprint(w, `<div id="tenant-detail" hx-swap-oob="innerHTML">`)
	partials.ExecuteTemplate(w, "tenant-detail-empty", nil)
	fmt.Fprint(w, `</div>`)
}

func buildDetailData(tenantID string, dbManager *db.TenantDBManager) uiDetailData {
	state, _ := dbManager.ReadState(tenantID)
	b, _ := json.MarshalIndent(state, "", "  ")
	_, isGlobal, _ := dbManager.TenantOwner(tenantID)
	return uiDetailData{
		TenantID:  tenantID,
		Name:      dbManager.TenantName(tenantID),
		PathID:    url.PathEscape(tenantID),
		StateJSON: string(b),
		IsGlobal:  isGlobal,
	}
}

func buildToolsData(tenantID string, dbManager *db.TenantDBManager, isGlobal bool) uiToolsTabData {
	pathID := url.PathEscape(tenantID)
	scenarios, _ := dbManager.GetScenariosForTenant(tenantID)
	var tenantTools []models.TenantScenario
	for _, s := range scenarios {
		if s.ToolName != "" {
			tenantTools = append(tenantTools, s)
		}
	}
	return uiToolsTabData{
		TenantID:    tenantID,
		Name:        dbManager.TenantName(tenantID),
		PathID:      pathID,
		TenantTools: tenantTools,
		IsGlobal:    isGlobal,
	}
}

func buildScenarioData(tenantID string, dbManager *db.TenantDBManager, isGlobal bool) uiScenarioTabData {
	active, _ := dbManager.GetActiveScenariosForTenant(tenantID)
	drafts, _ := dbManager.GetDraftScenariosForTenant(tenantID)
	return uiScenarioTabData{
		TenantID:       tenantID,
		Name:           dbManager.TenantName(tenantID),
		PathID:         url.PathEscape(tenantID),
		Scenarios:      active,
		DraftScenarios: drafts,
		IsGlobal:       isGlobal,
	}
}

func buildChaosData(tenantID string, chaosManager *chaos.Manager, isGlobal bool) uiChaosTabData {
	return uiChaosTabData{
		TenantID: tenantID,
		PathID:   url.PathEscape(tenantID),
		Profile:  string(chaosManager.GetProfile(tenantID)),
		Profiles: chaosProfiles,
		IsGlobal: isGlobal,
	}
}

// ── Tenant CRUD ───────────────────────────────────────────────────────────────

// GET /ui/tenants
func HandleUIGetTenants(dbManager *db.TenantDBManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := ""
		if u := UserFromContext(r); u != nil {
			userID = u.ID
		}
		uiSidebar(w, dbManager, userID)
	}
}

// POST /ui/tenants  form: name
func HandleUICreateTenant(dbManager *db.TenantDBManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := UserFromContext(r).ID
		name := strings.TrimSpace(r.FormValue("name"))
		if name == "" {
			name = strings.TrimSpace(r.FormValue("tenant_id")) // backward compat
		}
		if name == "" {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, `<span class="text-red-400 text-[11px]">✗ name required</span>`)
			return
		}
		tenantID, err := dbManager.CreateTenantForUser(name, userID)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, `<span class="text-red-400 text-[11px]">✗ %s</span>`, err.Error())
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		renderPartial(w, "tenant-detail-inner", buildDetailData(tenantID, dbManager))
		uiSidebarOOB(w, dbManager, userID)
	}
}

// DELETE /ui/tenants/{id}
func HandleUIDeleteTenant(dbManager *db.TenantDBManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := UserFromContext(r).ID
		tenantID := r.PathValue("id")
		if _, ok := resolveAccess(w, r, dbManager, tenantID, true); !ok {
			return
		}
		dbManager.DeleteState(tenantID)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		uiSidebar(w, dbManager, userID)
		uiDetailOOBEmpty(w)
	}
}

// GET /ui/tenants/{id}
func HandleUIGetTenantDetail(dbManager *db.TenantDBManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := r.PathValue("id")
		if _, ok := resolveAccess(w, r, dbManager, tenantID, false); !ok {
			return
		}
		renderPartial(w, "tenant-detail-inner", buildDetailData(tenantID, dbManager))
	}
}

// ── State tab ─────────────────────────────────────────────────────────────────

// GET /ui/tenants/{id}/state
func HandleUIGetStateTab(dbManager *db.TenantDBManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := r.PathValue("id")
		if _, ok := resolveAccess(w, r, dbManager, tenantID, false); !ok {
			return
		}
		renderPartial(w, "tenant-state-tab", buildDetailData(tenantID, dbManager))
	}
}

// PUT /ui/tenants/{id}/state  form: state (JSON string)
func HandleUISaveState(dbManager *db.TenantDBManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := r.PathValue("id")
		if _, ok := resolveAccess(w, r, dbManager, tenantID, true); !ok {
			return
		}
		raw := r.FormValue("state")
		var parsed map[string]interface{}
		if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			fmt.Fprintf(w, `<span class="text-red-400">✗ Invalid JSON: %s</span>`, err.Error())
			return
		}
		if err := dbManager.WriteState(tenantID, parsed); err != nil {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			fmt.Fprintf(w, `<span class="text-red-400">✗ %s</span>`, err.Error())
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `<span class="text-emerald-400">✓ Saved</span>`)
	}
}

// ── Scenarios tab ─────────────────────────────────────────────────────────────

// GET /ui/tenants/{id}/scenarios
func HandleUIGetScenariosTab(dbManager *db.TenantDBManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := r.PathValue("id")
		isGlobal, ok := resolveAccess(w, r, dbManager, tenantID, false)
		if !ok {
			return
		}
		renderPartial(w, "tenant-scenarios-tab", buildScenarioData(tenantID, dbManager, isGlobal))
	}
}

// POST /ui/tenants/{id}/scenarios  form: keywords, response
func HandleUICreateScenario(dbManager *db.TenantDBManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := r.PathValue("id")
		if _, ok := resolveAccess(w, r, dbManager, tenantID, true); !ok {
			return
		}
		keywords := splitKeywords(r.FormValue("keywords"))
		response := r.FormValue("response")
		if len(keywords) == 0 || response == "" {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			renderPartial(w, "tenant-scenario-items", buildScenarioData(tenantID, dbManager, false))
			return
		}
		dbManager.CreateScenario(models.TenantScenario{
			TenantID: tenantID, Keywords: keywords, Response: response,
		})
		renderPartial(w, "tenant-scenario-items", buildScenarioData(tenantID, dbManager, false))
	}
}

// DELETE /ui/tenants/{id}/scenarios/{sid}
func HandleUIDeleteScenario(dbManager *db.TenantDBManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := resolveAccess(w, r, dbManager, r.PathValue("id"), true); !ok {
			return
		}
		dbManager.DeleteScenario(r.PathValue("sid"))
		w.WriteHeader(http.StatusOK)
	}
}

// POST /ui/tenants/{id}/scenarios/{sid}/approve
func HandleUIApproveScenario(dbManager *db.TenantDBManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := r.PathValue("id")
		if _, ok := resolveAccess(w, r, dbManager, tenantID, true); !ok {
			return
		}
		dbManager.ApproveScenario(r.PathValue("sid"))
		renderPartial(w, "tenant-scenario-items", buildScenarioData(tenantID, dbManager, false))
	}
}

// ── Tools tab ─────────────────────────────────────────────────────────────────

// GET /ui/tenants/{id}/tools
func HandleUIGetToolsTab(dbManager *db.TenantDBManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := r.PathValue("id")
		isGlobal, ok := resolveAccess(w, r, dbManager, tenantID, false)
		if !ok {
			return
		}
		renderPartial(w, "tenant-tools-tab", buildToolsData(tenantID, dbManager, isGlobal))
	}
}

// POST /ui/tenants/{id}/tools  form: tool_name, keywords, response, tool_data
func HandleUICreateTool(dbManager *db.TenantDBManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := r.PathValue("id")
		if _, ok := resolveAccess(w, r, dbManager, tenantID, true); !ok {
			return
		}
		toolName := strings.TrimSpace(r.FormValue("tool_name"))
		if toolName == "" {
			renderPartial(w, "tenant-tool-items", buildToolsData(tenantID, dbManager, false))
			return
		}
		keywords := splitKeywords(r.FormValue("keywords"))
		response := r.FormValue("response")
		if response == "" {
			response = toolName
		}
		var toolData interface{}
		if raw := strings.TrimSpace(r.FormValue("tool_data")); raw != "" {
			json.Unmarshal([]byte(raw), &toolData)
		}
		dbManager.CreateScenario(models.TenantScenario{
			TenantID: tenantID, Keywords: keywords,
			Response: response, ToolName: toolName, ToolData: toolData,
		})
		renderPartial(w, "tenant-tool-items", buildToolsData(tenantID, dbManager, false))
	}
}

// DELETE /ui/tenants/{id}/tools/{sid}
func HandleUIDeleteTool(dbManager *db.TenantDBManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := resolveAccess(w, r, dbManager, r.PathValue("id"), true); !ok {
			return
		}
		dbManager.DeleteScenario(r.PathValue("sid"))
		w.WriteHeader(http.StatusOK)
	}
}

// ── Chaos tab ─────────────────────────────────────────────────────────────────

// GET /ui/tenants/{id}/chaos
func HandleUIGetChaosTab(chaosManager *chaos.Manager, dbManager *db.TenantDBManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := r.PathValue("id")
		isGlobal, ok := resolveAccess(w, r, dbManager, tenantID, false)
		if !ok {
			return
		}
		renderPartial(w, "tenant-chaos-tab", buildChaosData(tenantID, chaosManager, isGlobal))
	}
}

// POST /ui/tenants/{id}/chaos  vals: profile
func HandleUISetChaos(chaosManager *chaos.Manager, dbManager *db.TenantDBManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := r.PathValue("id")
		if _, ok := resolveAccess(w, r, dbManager, tenantID, true); !ok {
			return
		}
		profile := chaos.FaultProfile(r.FormValue("profile"))
		switch profile {
		case chaos.FaultNone, chaos.FaultRateLimit, chaos.FaultServerError, chaos.FaultLatency:
			chaosManager.SetProfile(tenantID, profile)
		}
		renderPartial(w, "tenant-chaos-tab", buildChaosData(tenantID, chaosManager, false))
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

func splitKeywords(s string) []string {
	var out []string
	for _, k := range strings.Split(s, ",") {
		if k = strings.TrimSpace(k); k != "" {
			out = append(out, k)
		}
	}
	return out
}
