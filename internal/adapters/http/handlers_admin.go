package http

import (
	"encoding/json"
	"log"
	"net/http"

	"llmplaceholder/internal/chaos"
	"llmplaceholder/internal/db"
)

func HandleSetChaos(chaosManager *chaos.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := r.Context().Value(TenantIDKey).(string)

		var body struct {
			Profile string `json:"profile"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		profile := chaos.FaultProfile(body.Profile)
		switch profile {
		case chaos.FaultNone, chaos.FaultRateLimit, chaos.FaultServerError, chaos.FaultLatency:
		default:
			http.Error(w, "Unknown profile", http.StatusBadRequest)
			return
		}

		chaosManager.SetProfile(tenantID, profile)
		log.Printf("[Admin] Chaos profile set to %q for Tenant: %s\n", profile, tenantID)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "success", "tenant": tenantID, "profile": string(profile)})
	}
}

func HandleListTenants(dbManager *db.TenantDBManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenants, err := dbManager.ListTenants()
		if err != nil {
			http.Error(w, "Failed to list tenants", http.StatusInternalServerError)
			return
		}
		if tenants == nil {
			tenants = []string{}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"tenants": tenants})
	}
}

// HandleResetTenant deletes the tenant's state so the next read reprovisions with empty state
func HandleResetTenant(dbManager *db.TenantDBManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := r.Context().Value(TenantIDKey).(string)

		log.Printf("[Admin] Resetting sandbox for Tenant: %s\n", tenantID)

		err := dbManager.DeleteState(tenantID)

		if err != nil {
			http.Error(w, "Failed to reset sandbox", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "success", "tenant": tenantID})
	}
}
