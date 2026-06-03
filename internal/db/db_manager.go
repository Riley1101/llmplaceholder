package db

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"sync"
)

// TenantDBManager orchestrates isolated flat-file databases
type TenantDBManager struct {
	mu           sync.RWMutex
	tenantLocks  map[string]*sync.RWMutex
	dataDir      string
	templatePath string
}

func NewTenantDBManager(dataDir string, templatePath string) *TenantDBManager {
	// Ensure the data directory exists
	os.MkdirAll(dataDir, 0755)

	return &TenantDBManager{
		tenantLocks:  make(map[string]*sync.RWMutex),
		dataDir:      dataDir,
		templatePath: templatePath,
	}
}

// getLock returns the specific mutex for a single tenant
func (m *TenantDBManager) getLock(tenantID string) *sync.RWMutex {
	m.mu.Lock()
	defer m.mu.Unlock()

	if lock, exists := m.tenantLocks[tenantID]; exists {
		return lock
	}
	m.tenantLocks[tenantID] = &sync.RWMutex{}
	return m.tenantLocks[tenantID]
}

// ReadState loads a tenant's database into a Go map
func (m *TenantDBManager) ReadState(tenantID string) (map[string]interface{}, error) {
	filePath := filepath.Join(m.dataDir, tenantID+".json")
	lock := m.getLock(tenantID)

	// Provision under write lock to prevent concurrent callers racing on a new file
	lock.Lock()
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		if err := m.provisionTenant(filePath); err != nil {
			lock.Unlock()
			return nil, err
		}
	}
	lock.Unlock()

	lock.RLock()
	defer lock.RUnlock()

	bytes, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var data map[string]interface{}
	err = json.Unmarshal(bytes, &data)
	return data, err
}

// WriteState serializes a Go map back to the tenant's isolated file
func (m *TenantDBManager) WriteState(tenantID string, data map[string]interface{}) error {
	lock := m.getLock(tenantID)
	lock.Lock() // Exclusive write lock
	defer lock.Unlock()

	filePath := filepath.Join(m.dataDir, tenantID+".json")
	bytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filePath, bytes, 0644)
}

func (m *TenantDBManager) provisionTenant(targetPath string) error {
	source, err := os.Open(m.templatePath)
	if err != nil {
		// Fallback to empty JSON object if template doesn't exist
		return os.WriteFile(targetPath, []byte("{}"), 0644)
	}
	defer source.Close()

	destination, err := os.Create(targetPath)
	if err != nil {
		return err
	}
	defer destination.Close()

	_, err = io.Copy(destination, source)
	return err
}
