package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"llmplaceholder/internal/core/models"

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

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS tenant_scenarios (
		id          TEXT PRIMARY KEY,
		tenant_id   TEXT NOT NULL,
		keywords    TEXT NOT NULL DEFAULT '[]',
		response    TEXT NOT NULL DEFAULT '',
		tool_name   TEXT NOT NULL DEFAULT '',
		tool_data   TEXT NOT NULL DEFAULT '{}',
		created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (tenant_id) REFERENCES tenant_state(tenant_id) ON DELETE CASCADE
	)`)
	if err != nil {
		return nil, err
	}

	return &TenantDBManager{db: db}, nil
}

// ── Tenant state ──────────────────────────────────────────────────────────────

func (m *TenantDBManager) TenantExists(tenantID string) (bool, error) {
	var count int
	err := m.db.QueryRow("SELECT COUNT(*) FROM tenant_state WHERE tenant_id = ?", tenantID).Scan(&count)
	return count > 0, err
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

// ── Tenant scenarios ──────────────────────────────────────────────────────────

func (m *TenantDBManager) GetScenariosForTenant(tenantID string) ([]models.TenantScenario, error) {
	rows, err := m.db.Query(
		"SELECT id, tenant_id, keywords, response, tool_name, tool_data FROM tenant_scenarios WHERE tenant_id = ? ORDER BY created_at",
		tenantID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var scenarios []models.TenantScenario
	for rows.Next() {
		var s models.TenantScenario
		var keywordsStr, toolDataStr string
		if err := rows.Scan(&s.ID, &s.TenantID, &keywordsStr, &s.Response, &s.ToolName, &toolDataStr); err != nil {
			return nil, err
		}
		json.Unmarshal([]byte(keywordsStr), &s.Keywords)
		if toolDataStr != "" && toolDataStr != "{}" {
			json.Unmarshal([]byte(toolDataStr), &s.ToolData)
		}
		scenarios = append(scenarios, s)
	}
	return scenarios, rows.Err()
}

func (m *TenantDBManager) CreateScenario(s models.TenantScenario) (models.TenantScenario, error) {
	s.ID = fmt.Sprintf("scn-%d", time.Now().UnixNano())

	keywordsJSON, _ := json.Marshal(s.Keywords)
	toolDataJSON := []byte("{}")
	if s.ToolData != nil {
		toolDataJSON, _ = json.Marshal(s.ToolData)
	}

	_, err := m.db.Exec(
		"INSERT INTO tenant_scenarios (id, tenant_id, keywords, response, tool_name, tool_data) VALUES (?, ?, ?, ?, ?, ?)",
		s.ID, s.TenantID, string(keywordsJSON), s.Response, s.ToolName, string(toolDataJSON),
	)
	return s, err
}

func (m *TenantDBManager) DeleteScenario(id string) error {
	_, err := m.db.Exec("DELETE FROM tenant_scenarios WHERE id = ?", id)
	return err
}

// ── Migration from legacy flat files ─────────────────────────────────────────

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
