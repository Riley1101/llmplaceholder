package chaos

import (
	"log"
	"net/http"
	"sync"
	"time"

	"llmplaceholder/internal/core/models"
)

// FaultProfile defines the type of chaos to inject
type FaultProfile string

const (
	FaultNone        FaultProfile = "none"
	FaultRateLimit   FaultProfile = "rate_limit"
	FaultServerError FaultProfile = "server_error"
	FaultLatency     FaultProfile = "latency"
)

// Manager keeps track of active chaos profiles per tenant in a thread-safe map
type Manager struct {
	mu       sync.RWMutex
	profiles map[string]FaultProfile
}

// NewManager initializes the chaos control plane
func NewManager() *Manager {
	return &Manager{
		profiles: make(map[string]FaultProfile),
	}
}

// SetProfile applies a specific fault to a tenant's sandbox
func (m *Manager) SetProfile(tenantID string, profile FaultProfile) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.profiles[tenantID] = profile
}

// GetProfile retrieves the active fault for a tenant
func (m *Manager) GetProfile(tenantID string) FaultProfile {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if profile, exists := m.profiles[tenantID]; exists {
		return profile
	}
	return FaultNone
}

// Middleware intercepts traffic and applies the configured fault before hitting the core engine
func (m *Manager) Middleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID, ok := r.Context().Value(models.TenantIDKey).(string)
		if !ok || tenantID == "" {
			tenantID = "default_local_user"
		}

		profile := m.GetProfile(tenantID)

		switch profile {
		case FaultRateLimit:
			log.Printf("[Chaos] Injecting 429 Rate Limit for Tenant: %s\n", tenantID)
			w.Header().Set("Retry-After", "30")
			w.Header().Set("Content-Type", "application/json")
			http.Error(w, `{"error": {"message": "Rate limit exceeded", "type": "requests", "param": null, "code": "rate_limit_exceeded"}}`, http.StatusTooManyRequests)
			return

		case FaultServerError:
			log.Printf("[Chaos] Injecting 500 Server Error for Tenant: %s\n", tenantID)
			w.Header().Set("Content-Type", "application/json")
			http.Error(w, `{"error": {"message": "Internal server error connecting to upstream GPU instances", "type": "server_error"}}`, http.StatusInternalServerError)
			return

		case FaultLatency:
			log.Printf("[Chaos] Injecting 5000ms Latency for Tenant: %s\n", tenantID)
			time.Sleep(5 * time.Second)
			next.ServeHTTP(w, r)
			return

		case FaultNone:
			fallthrough
		default:
			next.ServeHTTP(w, r)
		}
	}
}
