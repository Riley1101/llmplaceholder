package http

import (
	"context"
	"net/http"

	"llmplaceholder/internal/core/models"
)

// TenantIDKey re-exported for handler use within this package
const TenantIDKey = models.TenantIDKey

// CORSMiddleware sets CORS headers on every response and short-circuits OPTIONS preflight.
// Applied globally so admin and UI routes are also covered.
func CORSMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Tenant-ID")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// TenantMiddleware extracts the X-Tenant-ID header and injects it into the request context.
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
