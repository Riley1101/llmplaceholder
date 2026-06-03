package http

import (
	"encoding/json"
	"log"
	"net/http"

	"llmplaceholder/internal/db"
)

// HandleResetTenant clears a tenant's data file, forcing a fresh template copy
func HandleResetTenant(dbManager *db.TenantDBManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := r.Context().Value(TenantIDKey).(string)

		log.Printf("[Admin] Resetting sandbox for Tenant: %s\n", tenantID)

		// Overwrite the tenant file with a fresh empty map to trigger regeneration on next read
		err := dbManager.WriteState(tenantID, map[string]interface{}{})

		response := map[string]string{"status": "success", "tenant": tenantID}

		if err != nil {
			response["status"] = "error"
			response["message"] = err.Error()
			http.Error(w, "Failed to reset sandbox", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}
}
