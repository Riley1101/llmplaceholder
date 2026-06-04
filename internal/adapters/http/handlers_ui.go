package http

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"llmplaceholder/internal/chaos"
	"llmplaceholder/internal/core/models"
	"llmplaceholder/internal/core/registry"
	"llmplaceholder/internal/db"
)

var chaosProfiles = []string{"none", "rate_limit", "server_error", "latency"}

// ── data types ────────────────────────────────────────────────────────────────

type uiDetailData struct {
	TenantID  string
	PathID    string
	StateJSON string
}

type uiScenarioTabData struct {
	TenantID  string
	PathID    string
	Scenarios []models.TenantScenario
	MockPage  uiMockScenariosData
}

type uiMockScenariosData struct {
	PathID     string
	Scenarios  []models.MockScenario
	Page       int
	TotalPages int
	PrevPage   int
	NextPage   int
	HasPrev    bool
	HasNext    bool
}

type uiToolsTabData struct {
	TenantID      string
	PathID        string
	TenantTools   []models.TenantScenario
	GlobalTools   []string
	IncludeGlobal bool
}

type uiChaosTabData struct {
	TenantID string
	PathID   string
	Profile  string
	Profiles []string
}

// ── helpers ───────────────────────────────────────────────────────────────────

func uiSidebar(w http.ResponseWriter, dbManager *db.TenantDBManager) {
	tenants, _ := dbManager.ListTenants()
	renderPartial(w, "tenant-sidebar", sidebarData{Tenants: tenants})
}

func uiSidebarOOB(w http.ResponseWriter, dbManager *db.TenantDBManager) {
	tenants, _ := dbManager.ListTenants()
	fmt.Fprint(w, `<div id="tenant-sidebar-list" hx-swap-oob="innerHTML">`)
	partials.ExecuteTemplate(w, "tenant-sidebar", sidebarData{Tenants: tenants})
	fmt.Fprint(w, `</div>`)
}

func uiDetailOOBEmpty(w http.ResponseWriter) {
	fmt.Fprint(w, `<div id="tenant-detail" hx-swap-oob="innerHTML">`)
	partials.ExecuteTemplate(w, "tenant-detail-empty", nil)
	fmt.Fprint(w, `</div>`)
}

func buildDetailData(tenantID string, dbManager *db.TenantDBManager) uiDetailData {
	state, _ := dbManager.ReadState(tenantID)
	b, _ := json.MarshalIndent(state, "", "  ")
	return uiDetailData{
		TenantID:  tenantID,
		PathID:    url.PathEscape(tenantID),
		StateJSON: string(b),
	}
}

func buildToolsData(tenantID string, dbManager *db.TenantDBManager) uiToolsTabData {
	pathID := url.PathEscape(tenantID)
	scenarios, _ := dbManager.GetScenariosForTenant(tenantID)
	settings, _ := dbManager.ReadSettings(tenantID)

	includeGlobal := true
	if v, ok := settings["include_global_tools"]; ok {
		if b, ok := v.(bool); ok {
			includeGlobal = b
		}
	}

	var tenantTools []models.TenantScenario
	for _, s := range scenarios {
		if s.ToolName != "" {
			tenantTools = append(tenantTools, s)
		}
	}

	var globalNames []string
	if includeGlobal {
		for _, t := range registry.ListToolsForTenant(nil, true) {
			globalNames = append(globalNames, t["name"])
		}
	}

	return uiToolsTabData{
		TenantID:      tenantID,
		PathID:        pathID,
		TenantTools:   tenantTools,
		GlobalTools:   globalNames,
		IncludeGlobal: includeGlobal,
	}
}

// ── Tenant CRUD ───────────────────────────────────────────────────────────────

// GET /ui/tenants
func HandleUIGetTenants(dbManager *db.TenantDBManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uiSidebar(w, dbManager)
	}
}

// POST /ui/tenants  form: tenant_id
func HandleUICreateTenant(dbManager *db.TenantDBManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := strings.TrimSpace(r.FormValue("tenant_id"))
		if tenantID == "" {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, `<span class="text-red-400 text-[11px]">✗ tenant_id required</span>`)
			return
		}
		if err := dbManager.WriteState(tenantID, map[string]interface{}{}); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, `<span class="text-red-400 text-[11px]">✗ %s</span>`, err.Error())
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		// primary: detail panel for the new tenant
		renderPartial(w, "tenant-detail-inner", buildDetailData(tenantID, dbManager))
		// OOB: refresh sidebar
		uiSidebarOOB(w, dbManager)
	}
}

// DELETE /ui/tenants/{id}
func HandleUIDeleteTenant(dbManager *db.TenantDBManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		dbManager.DeleteState(r.PathValue("id"))
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		// primary: updated sidebar
		uiSidebar(w, dbManager)
		// OOB: clear detail
		uiDetailOOBEmpty(w)
	}
}

// GET /ui/tenants/{id}
func HandleUIGetTenantDetail(dbManager *db.TenantDBManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		renderPartial(w, "tenant-detail-inner", buildDetailData(r.PathValue("id"), dbManager))
	}
}

// ── State tab ─────────────────────────────────────────────────────────────────

// GET /ui/tenants/{id}/state
func HandleUIGetStateTab(dbManager *db.TenantDBManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		renderPartial(w, "tenant-state-tab", buildDetailData(r.PathValue("id"), dbManager))
	}
}

// PUT /ui/tenants/{id}/state  form: state (JSON string)
func HandleUISaveState(dbManager *db.TenantDBManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := r.PathValue("id")
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

// domainAliases maps scenario ID prefixes to tenant name keywords that should match them.
var domainAliases = map[string][]string{
	"fintech":  {"ecommerce", "fintech", "billing", "invoice"},
	"saas":     {"saas"},
	"devops":   {"devops", "ops", "sre"},
	"homelab":  {"homelab", "home"},
	"kernel":   {"kernel", "os"},
	"legal":    {"legal", "law"},
	"retail":   {"retail", "store", "shop"},
	"security": {"security", "soc", "sec"},
}

func mockScenariosForTenant(tenantID string) []models.MockScenario {
	tid := strings.ToLower(tenantID)
	var matched []models.MockScenario
	for _, s := range registry.GlobalRegistry {
		if len(s.Keywords) == 0 {
			continue
		}
		prefix := s.ID
		if i := strings.Index(s.ID, "_"); i > 0 {
			prefix = s.ID[:i]
		}
		aliases := domainAliases[prefix]
		if aliases == nil {
			aliases = []string{prefix}
		}
		for _, a := range aliases {
			if strings.Contains(tid, a) {
				matched = append(matched, s)
				break
			}
		}
	}
	if len(matched) == 0 {
		for _, s := range registry.GlobalRegistry {
			if len(s.Keywords) > 0 {
				matched = append(matched, s)
			}
		}
	}
	return matched
}

const mockPageSize = 10

func buildMockPage(tenantID string, page int) uiMockScenariosData {
	all := mockScenariosForTenant(tenantID)
	total := len(all)
	totalPages := (total + mockPageSize - 1) / mockPageSize
	if totalPages == 0 {
		totalPages = 1
	}
	if page < 0 {
		page = 0
	}
	if page >= totalPages {
		page = totalPages - 1
	}
	start := page * mockPageSize
	end := start + mockPageSize
	if end > total {
		end = total
	}
	return uiMockScenariosData{
		PathID:     url.PathEscape(tenantID),
		Scenarios:  all[start:end],
		Page:       page,
		TotalPages: totalPages,
		PrevPage:   page - 1,
		NextPage:   page + 1,
		HasPrev:    page > 0,
		HasNext:    page < totalPages-1,
	}
}

func buildScenarioData(tenantID string, dbManager *db.TenantDBManager) uiScenarioTabData {
	scenarios, _ := dbManager.GetScenariosForTenant(tenantID)
	return uiScenarioTabData{
		TenantID:  tenantID,
		PathID:    url.PathEscape(tenantID),
		Scenarios: scenarios,
		MockPage:  buildMockPage(tenantID, 0),
	}
}

// GET /ui/tenants/{id}/mock-scenarios?page=N
func HandleUIGetMockScenarios() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := r.PathValue("id")
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		renderPartial(w, "tenant-mock-scenarios", buildMockPage(tenantID, page))
	}
}

// GET /ui/global-scenarios?page=N
func HandleUIGetGlobalScenarios() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		renderPartial(w, "global-scenarios-list", buildGlobalScenariosPage(page))
	}
}

// POST /ui/global-scenarios  form: keywords, response
func HandleUICreateGlobalScenario() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "bad form", http.StatusBadRequest)
			return
		}
		raw := r.FormValue("keywords")
		response := strings.TrimSpace(r.FormValue("response"))
		if raw == "" || response == "" {
			http.Error(w, "keywords and response required", http.StatusBadRequest)
			return
		}
		var kws []string
		for _, k := range strings.Split(raw, ",") {
			if k = strings.TrimSpace(k); k != "" {
				kws = append(kws, strings.ToLower(k))
			}
		}
		id := "custom_" + strings.ReplaceAll(uuid.New().String(), "-", "")[:8]
		registry.AddGlobalScenario(models.MockScenario{
			ID:           id,
			Keywords:     kws,
			FullResponse: response,
		})
		renderPartial(w, "global-scenarios-list", buildGlobalScenariosPage(0))
	}
}

// DELETE /ui/global-scenarios/{id}
func HandleUIDeleteGlobalScenario() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		registry.DeleteGlobalScenario(r.PathValue("id"))
		w.WriteHeader(http.StatusOK)
	}
}

func buildGlobalScenariosPage(page int) uiMockScenariosData {
	all := make([]models.MockScenario, 0)
	for _, s := range registry.GlobalRegistry {
		if len(s.Keywords) > 0 {
			all = append(all, s)
		}
	}
	total := len(all)
	totalPages := (total + mockPageSize - 1) / mockPageSize
	if totalPages == 0 {
		totalPages = 1
	}
	if page < 0 {
		page = 0
	}
	if page >= totalPages {
		page = totalPages - 1
	}
	start := page * mockPageSize
	end := start + mockPageSize
	if end > total {
		end = total
	}
	return uiMockScenariosData{
		Scenarios:  all[start:end],
		Page:       page,
		TotalPages: totalPages,
		PrevPage:   page - 1,
		NextPage:   page + 1,
		HasPrev:    page > 0,
		HasNext:    page < totalPages-1,
	}
}

// GET /ui/tenants/{id}/scenarios
func HandleUIGetScenariosTab(dbManager *db.TenantDBManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		renderPartial(w, "tenant-scenarios-tab", buildScenarioData(r.PathValue("id"), dbManager))
	}
}

// POST /ui/tenants/{id}/scenarios  form: keywords, response
func HandleUICreateScenario(dbManager *db.TenantDBManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := r.PathValue("id")
		keywords := splitKeywords(r.FormValue("keywords"))
		response := r.FormValue("response")
		if len(keywords) == 0 || response == "" {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			// return updated list unchanged, show msg via hx-swap-oob
			renderPartial(w, "tenant-scenario-items", buildScenarioData(tenantID, dbManager))
			return
		}
		dbManager.CreateScenario(models.TenantScenario{
			TenantID: tenantID, Keywords: keywords, Response: response,
		})
		renderPartial(w, "tenant-scenario-items", buildScenarioData(tenantID, dbManager))
	}
}

// DELETE /ui/tenants/{id}/scenarios/{sid}
func HandleUIDeleteScenario(dbManager *db.TenantDBManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		dbManager.DeleteScenario(r.PathValue("sid"))
		w.WriteHeader(http.StatusOK)
	}
}

// ── Tools tab ─────────────────────────────────────────────────────────────────

// GET /ui/tenants/{id}/tools
func HandleUIGetToolsTab(dbManager *db.TenantDBManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		renderPartial(w, "tenant-tools-tab", buildToolsData(r.PathValue("id"), dbManager))
	}
}

// POST /ui/tenants/{id}/tools  form: tool_name, keywords, response, tool_data
func HandleUICreateTool(dbManager *db.TenantDBManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := r.PathValue("id")
		toolName := strings.TrimSpace(r.FormValue("tool_name"))
		if toolName == "" {
			renderPartial(w, "tenant-tool-items", buildToolsData(tenantID, dbManager))
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
		renderPartial(w, "tenant-tool-items", buildToolsData(tenantID, dbManager))
	}
}

// DELETE /ui/tenants/{id}/tools/{sid}
func HandleUIDeleteTool(dbManager *db.TenantDBManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		dbManager.DeleteScenario(r.PathValue("sid"))
		w.WriteHeader(http.StatusOK)
	}
}

// POST /ui/tenants/{id}/settings  form: include_global_tools
func HandleUIToggleSettings(dbManager *db.TenantDBManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := r.PathValue("id")
		val := r.FormValue("include_global_tools") == "true"
		existing, _ := dbManager.ReadSettings(tenantID)
		existing["include_global_tools"] = val
		dbManager.WriteSettings(tenantID, existing)
		renderPartial(w, "tenant-tools-tab", buildToolsData(tenantID, dbManager))
	}
}

// ── Chaos tab ─────────────────────────────────────────────────────────────────

func buildChaosData(tenantID string, chaosManager *chaos.Manager) uiChaosTabData {
	return uiChaosTabData{
		TenantID: tenantID,
		PathID:   url.PathEscape(tenantID),
		Profile:  string(chaosManager.GetProfile(tenantID)),
		Profiles: chaosProfiles,
	}
}

// GET /ui/tenants/{id}/chaos
func HandleUIGetChaosTab(chaosManager *chaos.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		renderPartial(w, "tenant-chaos-tab", buildChaosData(r.PathValue("id"), chaosManager))
	}
}

// POST /ui/tenants/{id}/chaos  vals: profile
func HandleUISetChaos(chaosManager *chaos.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := r.PathValue("id")
		profile := chaos.FaultProfile(r.FormValue("profile"))
		switch profile {
		case chaos.FaultNone, chaos.FaultRateLimit, chaos.FaultServerError, chaos.FaultLatency:
			chaosManager.SetProfile(tenantID, profile)
		}
		renderPartial(w, "tenant-chaos-tab", buildChaosData(tenantID, chaosManager))
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
