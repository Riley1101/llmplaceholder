package db

import (
	"database/sql"
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

type TenantDBManager struct {
	db *sql.DB
}

func NewTenantDBManager(dbPath string) (*TenantDBManager, error) {
	os.MkdirAll(filepath.Dir(dbPath), 0755)

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}

	db.Exec("PRAGMA journal_mode=WAL")
	db.Exec("PRAGMA foreign_keys=ON")

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS tenant_state (
		tenant_id   TEXT PRIMARY KEY,
		state_json  TEXT NOT NULL DEFAULT '{}',
		created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at  DATETIME DEFAULT CURRENT_TIMESTAMP
	)`)
	if err != nil {
		return nil, err
	}

	return &TenantDBManager{db: db}, nil
}

func (m *TenantDBManager) ReadState(tenantID string) (map[string]interface{}, error) {
	var raw string
	err := m.db.QueryRow("SELECT state_json FROM tenant_state WHERE tenant_id = ?", tenantID).Scan(&raw)
	if err == sql.ErrNoRows {
		if err := m.provisionTenant(tenantID); err != nil {
			return nil, err
		}
		return map[string]interface{}{}, nil
	}
	if err != nil {
		return nil, err
	}

	var data map[string]interface{}
	err = json.Unmarshal([]byte(raw), &data)
	return data, err
}

func (m *TenantDBManager) WriteState(tenantID string, data map[string]interface{}) error {
	bytes, err := json.Marshal(data)
	if err != nil {
		return err
	}

	_, err = m.db.Exec(`
		INSERT INTO tenant_state (tenant_id, state_json, updated_at)
		VALUES (?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(tenant_id) DO UPDATE SET
			state_json = excluded.state_json,
			updated_at = CURRENT_TIMESTAMP`,
		tenantID, string(bytes))
	return err
}

func (m *TenantDBManager) DeleteState(tenantID string) error {
	_, err := m.db.Exec("DELETE FROM tenant_state WHERE tenant_id = ?", tenantID)
	return err
}

func (m *TenantDBManager) ListTenants() ([]string, error) {
	rows, err := m.db.Query("SELECT tenant_id FROM tenant_state ORDER BY created_at")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tenants []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		tenants = append(tenants, id)
	}
	return tenants, rows.Err()
}

func (m *TenantDBManager) provisionTenant(tenantID string) error {
	_, err := m.db.Exec("INSERT OR IGNORE INTO tenant_state (tenant_id, state_json) VALUES (?, '{}')", tenantID)
	return err
}

// MigrateFromFiles seeds SQLite from legacy JSON files. No-op if dir doesn't exist or tenant already present.
func (m *TenantDBManager) MigrateFromFiles(dataDir string) {
	entries, err := os.ReadDir(dataDir)
	if err != nil {
		return
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		tenantID := strings.TrimSuffix(entry.Name(), ".json")

		var count int
		m.db.QueryRow("SELECT COUNT(*) FROM tenant_state WHERE tenant_id = ?", tenantID).Scan(&count)
		if count > 0 {
			continue
		}

		data, err := os.ReadFile(filepath.Join(dataDir, entry.Name()))
		if err != nil {
			log.Printf("[DB] Migration skip %s: %v", entry.Name(), err)
			continue
		}

		_, err = m.db.Exec("INSERT OR IGNORE INTO tenant_state (tenant_id, state_json) VALUES (?, ?)", tenantID, string(data))
		if err != nil {
			log.Printf("[DB] Migration failed %s: %v", tenantID, err)
			continue
		}
		log.Printf("[DB] Migrated tenant: %s", tenantID)
	}
}
