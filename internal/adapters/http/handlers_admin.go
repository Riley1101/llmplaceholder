package http

import (
	"encoding/json"
	"log"
	"net/http"

	"llmplaceholder/internal/chaos"
	"llmplaceholder/internal/core/models"
	"llmplaceholder/internal/db"
)

func jsonOK(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func jsonErr(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

// ── Tenant CRUD ───────────────────────────────────────────────────────────────

// GET /admin/tenants
func HandleListTenants(dbManager *db.TenantDBManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenants, err := dbManager.ListTenants()
		if err != nil {
			jsonErr(w, "failed to list tenants", http.StatusInternalServerError)
			return
		}
		if tenants == nil {
			tenants = []string{}
		}
		jsonOK(w, map[string]interface{}{"tenants": tenants})
	}
}

// POST /admin/tenants  body: {"tenant_id":"...", "state":{...}}
func HandleCreateTenant(dbManager *db.TenantDBManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			TenantID string                 `json:"tenant_id"`
			State    map[string]interface{} `json:"state"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.TenantID == "" {
			jsonErr(w, "invalid body: tenant_id required", http.StatusBadRequest)
			return
		}

		exists, err := dbManager.TenantExists(body.TenantID)
		if err != nil {
			jsonErr(w, "db error", http.StatusInternalServerError)
			return
		}
		if exists {
			jsonErr(w, "tenant already exists", http.StatusConflict)
			return
		}

		if body.State == nil {
			body.State = map[string]interface{}{}
		}
		if err := dbManager.WriteState(body.TenantID, body.State); err != nil {
			jsonErr(w, "failed to create tenant", http.StatusInternalServerError)
			return
		}

		log.Printf("[Admin] Created tenant: %s\n", body.TenantID)
		w.WriteHeader(http.StatusCreated)
		jsonOK(w, map[string]string{"status": "created", "tenant_id": body.TenantID})
	}
}

// GET /admin/tenants/{id}
func HandleGetTenant(dbManager *db.TenantDBManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := r.PathValue("id")
		state, err := dbManager.ReadState(tenantID)
		if err != nil {
			jsonErr(w, "failed to read state", http.StatusInternalServerError)
			return
		}
		jsonOK(w, map[string]interface{}{"tenant_id": tenantID, "state": state})
	}
}

// PUT /admin/tenants/{id}/state  body: {"state":{...}}
func HandleUpdateTenantState(dbManager *db.TenantDBManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := r.PathValue("id")

		var body struct {
			State map[string]interface{} `json:"state"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.State == nil {
			jsonErr(w, "invalid body: state object required", http.StatusBadRequest)
			return
		}

		if err := dbManager.WriteState(tenantID, body.State); err != nil {
			jsonErr(w, "failed to update state", http.StatusInternalServerError)
			return
		}

		jsonOK(w, map[string]string{"status": "updated", "tenant_id": tenantID})
	}
}

// DELETE /admin/tenants/{id}
func HandleDeleteTenant(dbManager *db.TenantDBManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := r.PathValue("id")
		if err := dbManager.DeleteState(tenantID); err != nil {
			jsonErr(w, "failed to delete tenant", http.StatusInternalServerError)
			return
		}
		log.Printf("[Admin] Deleted tenant: %s\n", tenantID)
		jsonOK(w, map[string]string{"status": "deleted", "tenant_id": tenantID})
	}
}

// POST /admin/reset  (header-based, kept for backward compat)
func HandleResetTenant(dbManager *db.TenantDBManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := r.Context().Value(TenantIDKey).(string)
		log.Printf("[Admin] Resetting sandbox for Tenant: %s\n", tenantID)
		if err := dbManager.DeleteState(tenantID); err != nil {
			jsonErr(w, "failed to reset sandbox", http.StatusInternalServerError)
			return
		}
		jsonOK(w, map[string]string{"status": "reset", "tenant": tenantID})
	}
}

// ── Scenario CRUD ─────────────────────────────────────────────────────────────

// GET /admin/tenants/{id}/scenarios
func HandleListScenarios(dbManager *db.TenantDBManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := r.PathValue("id")
		scenarios, err := dbManager.GetScenariosForTenant(tenantID)
		if err != nil {
			jsonErr(w, "failed to list scenarios", http.StatusInternalServerError)
			return
		}
		if scenarios == nil {
			scenarios = []models.TenantScenario{}
		}
		jsonOK(w, map[string]interface{}{"tenant_id": tenantID, "scenarios": scenarios})
	}
}

// POST /admin/tenants/{id}/scenarios
// body: {"keywords":["..."],"response":"...","tool_name":"...","tool_data":{...}}
func HandleCreateScenario(dbManager *db.TenantDBManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := r.PathValue("id")

		var body models.TenantScenario
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			jsonErr(w, "invalid body", http.StatusBadRequest)
			return
		}
		if len(body.Keywords) == 0 || body.Response == "" {
			jsonErr(w, "keywords and response are required", http.StatusBadRequest)
			return
		}
		body.TenantID = tenantID

		// Ensure tenant row exists before inserting FK-constrained scenario
		if _, err := dbManager.ReadState(tenantID); err != nil {
			jsonErr(w, "tenant not found", http.StatusNotFound)
			return
		}

		created, err := dbManager.CreateScenario(body)
		if err != nil {
			jsonErr(w, "failed to create scenario", http.StatusInternalServerError)
			return
		}

		log.Printf("[Admin] Created scenario %s for tenant %s\n", created.ID, tenantID)
		w.WriteHeader(http.StatusCreated)
		jsonOK(w, created)
	}
}

// DELETE /admin/tenants/{id}/scenarios/{sid}
func HandleDeleteScenario(dbManager *db.TenantDBManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sid := r.PathValue("sid")
		if err := dbManager.DeleteScenario(sid); err != nil {
			jsonErr(w, "failed to delete scenario", http.StatusInternalServerError)
			return
		}
		jsonOK(w, map[string]string{"status": "deleted", "id": sid})
	}
}

// ── Tenant settings ───────────────────────────────────────────────────────────

// GET /admin/tenants/{id}/settings
func HandleGetTenantSettings(dbManager *db.TenantDBManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := r.PathValue("id")
		settings, err := dbManager.ReadSettings(tenantID)
		if err != nil {
			jsonErr(w, "failed to read settings", http.StatusInternalServerError)
			return
		}
		jsonOK(w, map[string]interface{}{"tenant_id": tenantID, "settings": settings})
	}
}

// PATCH /admin/tenants/{id}/settings  body: {"include_global_tools": false, ...}
func HandlePatchTenantSettings(dbManager *db.TenantDBManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := r.PathValue("id")
		var patch map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
			jsonErr(w, "invalid JSON", http.StatusBadRequest)
			return
		}
		existing, _ := dbManager.ReadSettings(tenantID)
		for k, v := range patch {
			existing[k] = v
		}
		if err := dbManager.WriteSettings(tenantID, existing); err != nil {
			jsonErr(w, "failed to save settings", http.StatusInternalServerError)
			return
		}
		jsonOK(w, map[string]interface{}{"tenant_id": tenantID, "settings": existing})
	}
}

// ── Chaos ─────────────────────────────────────────────────────────────────────

// POST /admin/chaos  (header-based)
func HandleSetChaos(chaosManager *chaos.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := r.Context().Value(TenantIDKey).(string)

		var body struct {
			Profile string `json:"profile"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			jsonErr(w, "invalid JSON", http.StatusBadRequest)
			return
		}

		profile := chaos.FaultProfile(body.Profile)
		switch profile {
		case chaos.FaultNone, chaos.FaultRateLimit, chaos.FaultServerError, chaos.FaultLatency:
		default:
			jsonErr(w, "unknown profile", http.StatusBadRequest)
			return
		}

		chaosManager.SetProfile(tenantID, profile)
		log.Printf("[Admin] Chaos profile set to %q for Tenant: %s\n", profile, tenantID)
		jsonOK(w, map[string]string{"status": "ok", "tenant": tenantID, "profile": string(profile)})
	}
}

// POST /admin/tenants/{id}/chaos  body: {"profile":"..."}
func HandleSetTenantChaos(chaosManager *chaos.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := r.PathValue("id")

		var body struct {
			Profile string `json:"profile"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			jsonErr(w, "invalid JSON", http.StatusBadRequest)
			return
		}

		profile := chaos.FaultProfile(body.Profile)
		switch profile {
		case chaos.FaultNone, chaos.FaultRateLimit, chaos.FaultServerError, chaos.FaultLatency:
		default:
			jsonErr(w, "unknown profile", http.StatusBadRequest)
			return
		}

		chaosManager.SetProfile(tenantID, profile)
		jsonOK(w, map[string]string{"status": "ok", "tenant_id": tenantID, "profile": string(profile)})
	}
}

// GET /admin/tenants/{id}/chaos
func HandleGetTenantChaos(chaosManager *chaos.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := r.PathValue("id")
		profile := chaosManager.GetProfile(tenantID)
		jsonOK(w, map[string]string{"tenant_id": tenantID, "profile": string(profile)})
	}
}
