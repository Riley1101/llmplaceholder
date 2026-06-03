package http

import (
	"encoding/json"
	"log"
	"net/http"

	"llmplaceholder/internal/db"
)

// HandleResetTenant deletes the tenant's data file so the next read reprovisions from template
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
