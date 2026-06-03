package http

import (
	"context"
	"net/http"

	"llmplaceholder/internal/core/models"
)

// TenantIDKey re-exported for handler use within this package
const TenantIDKey = models.TenantIDKey

// TenantMiddleware extracts the X-Tenant-ID header and injects it into the request context.
// This ensures that all downstream handlers and the DB manager isolate data per user.
func TenantMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := r.Header.Get("X-Tenant-ID")

		if tenantID == "" {
			tenantID = "default_local_user"
		}

		ctx := context.WithValue(r.Context(), models.TenantIDKey, tenantID)
		next.ServeHTTP(w, r.WithContext(ctx))
	}
}
