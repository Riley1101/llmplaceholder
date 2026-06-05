package http

import (
	"context"
	"net/http"
	"strings"

	"llmplaceholder/internal/core/models"
	"llmplaceholder/internal/db"
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
// Falls back to "default_local_user" when the header is absent.
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

// StrictTenantMiddleware rejects requests that do not include the X-Tenant-ID header.
func StrictTenantMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := r.Header.Get("X-Tenant-ID")
		if tenantID == "" {
			w.Header().Set("Content-Type", "application/json")
			http.Error(w, `{"error":"X-Tenant-ID header is required"}`, http.StatusBadRequest)
			return
		}
		ctx := context.WithValue(r.Context(), models.TenantIDKey, tenantID)
		next.ServeHTTP(w, r.WithContext(ctx))
	}
}

type userContextKey struct{}

// UserFromContext returns the authenticated user from the request context, or nil.
func UserFromContext(r *http.Request) *models.User {
	u, _ := r.Context().Value(userContextKey{}).(*models.User)
	return u
}

// AuthMiddleware resolves the caller identity from either a Bearer token or session cookie.
// Applied globally — routes that require auth should also use RequireAuth.
func AuthMiddleware(dbManager *db.TenantDBManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 1. Bearer token (programmatic access)
			if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
				token := strings.TrimPrefix(auth, "Bearer ")
				if u, err := dbManager.GetUserByToken(token); err == nil && u != nil {
					ctx := context.WithValue(r.Context(), userContextKey{}, u)
					next.ServeHTTP(w, r.WithContext(ctx))
					return
				}
			}

			// 2. Session cookie (browser)
			if cookie, err := r.Cookie(sessionCookieName); err == nil {
				if u, err := dbManager.GetSessionUser(cookie.Value); err == nil && u != nil {
					ctx := context.WithValue(r.Context(), userContextKey{}, u)
					r = r.WithContext(ctx)
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RequireAuth blocks unauthenticated requests.
// HTMX requests get HX-Redirect; JSON requests get 401; others redirect to /login.
func RequireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if UserFromContext(r) == nil {
			if r.Header.Get("HX-Request") == "true" {
				w.Header().Set("HX-Redirect", "/login")
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			if strings.Contains(r.Header.Get("Accept"), "application/json") {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				w.Write([]byte(`{"error":"unauthorized"}`))
				return
			}
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}
		next.ServeHTTP(w, r)
	}
}
