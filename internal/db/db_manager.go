package db

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"llmplaceholder/internal/core/models"

	turso "turso.tech/database/tursogo"
	_ "modernc.org/sqlite"
)

type TenantDBManager struct {
	db *sql.DB
}

func NewTenantDBManager(dbPath string) (*TenantDBManager, error) {
	os.MkdirAll(filepath.Dir(dbPath), 0755)

	var db *sql.DB
	var err error

	tursoURL := os.Getenv("TURSO_DATABASE_URL")
	tursoToken := os.Getenv("TURSO_AUTH_TOKEN")

	if tursoURL != "" && tursoToken != "" {
		bootstrap := true
		var syncDB *turso.TursoSyncDb
		syncDB, err = turso.NewTursoSyncDb(context.Background(), turso.TursoSyncDbConfig{
			Path:             dbPath,
			RemoteUrl:        tursoURL,
			AuthToken:        tursoToken,
			BootstrapIfEmpty: &bootstrap,
		})
		if err != nil {
			return nil, fmt.Errorf("turso sync init: %w", err)
		}
		db, err = syncDB.Connect(context.Background())
		if err != nil {
			return nil, fmt.Errorf("turso connect: %w", err)
		}
		log.Println("[DB] Using Turso remote:", tursoURL)
	} else {
		db, err = sql.Open("sqlite", dbPath)
		if err != nil {
			return nil, err
		}
		log.Println("[DB] Using local SQLite:", dbPath)
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

	// idempotent migrations — ignored if column already exists
	db.Exec(`ALTER TABLE tenant_state ADD COLUMN settings_json TEXT NOT NULL DEFAULT '{}'`)
	db.Exec(`ALTER TABLE tenant_state ADD COLUMN owner_id TEXT REFERENCES users(id)`)

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS users (
		id          TEXT PRIMARY KEY,
		github_id   INTEGER UNIQUE NOT NULL,
		login       TEXT NOT NULL,
		name        TEXT NOT NULL DEFAULT '',
		email       TEXT NOT NULL DEFAULT '',
		avatar_url  TEXT NOT NULL DEFAULT '',
		created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at  DATETIME DEFAULT CURRENT_TIMESTAMP
	)`)
	if err != nil {
		return nil, err
	}

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS sessions (
		id          TEXT PRIMARY KEY,
		user_id     TEXT NOT NULL,
		expires_at  DATETIME NOT NULL,
		created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
	)`)
	if err != nil {
		return nil, err
	}

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS api_tokens (
		id           TEXT PRIMARY KEY,
		user_id      TEXT NOT NULL,
		name         TEXT NOT NULL,
		token_hash   TEXT UNIQUE NOT NULL,
		created_at   DATETIME DEFAULT CURRENT_TIMESTAMP,
		last_used_at DATETIME,
		FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
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

// ── Tenant settings ───────────────────────────────────────────────────────────

func (m *TenantDBManager) ReadSettings(tenantID string) (map[string]interface{}, error) {
	var raw string
	err := m.db.QueryRow("SELECT settings_json FROM tenant_state WHERE tenant_id = ?", tenantID).Scan(&raw)
	if err != nil {
		return map[string]interface{}{}, nil
	}
	var data map[string]interface{}
	json.Unmarshal([]byte(raw), &data)
	return data, nil
}

func (m *TenantDBManager) WriteSettings(tenantID string, settings map[string]interface{}) error {
	b, _ := json.Marshal(settings)
	_, err := m.db.Exec(`UPDATE tenant_state SET settings_json = ? WHERE tenant_id = ?`, string(b), tenantID)
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

// ── Tenant ownership ──────────────────────────────────────────────────────────

// ListTenantsForUser returns tenants owned by userID plus all global tenants (owner_id IS NULL).
func (m *TenantDBManager) ListTenantsForUser(userID string) ([]models.TenantMeta, error) {
	rows, err := m.db.Query(
		`SELECT tenant_id, (owner_id IS NULL) as is_global
		 FROM tenant_state
		 WHERE owner_id = ? OR owner_id IS NULL
		 ORDER BY (owner_id IS NULL), created_at`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tenants []models.TenantMeta
	for rows.Next() {
		var t models.TenantMeta
		var isGlobal int
		if err := rows.Scan(&t.ID, &isGlobal); err != nil {
			return nil, err
		}
		t.IsGlobal = isGlobal == 1
		tenants = append(tenants, t)
	}
	return tenants, rows.Err()
}

// CreateTenantForUser inserts a new tenant owned by userID.
// Returns an error if tenant_id already exists.
func (m *TenantDBManager) CreateTenantForUser(tenantID, userID string) error {
	_, err := m.db.Exec(
		`INSERT INTO tenant_state (tenant_id, state_json, owner_id) VALUES (?, '{}', ?)`,
		tenantID, userID)
	return err
}

// TenantOwner returns the ownerID for a tenant plus whether it is global (owner_id IS NULL).
// Returns sql.ErrNoRows if the tenant does not exist.
func (m *TenantDBManager) TenantOwner(tenantID string) (ownerID string, isGlobal bool, err error) {
	var raw sql.NullString
	err = m.db.QueryRow("SELECT owner_id FROM tenant_state WHERE tenant_id = ?", tenantID).Scan(&raw)
	if err != nil {
		return "", false, err
	}
	if !raw.Valid {
		return "", true, nil
	}
	return raw.String, false, nil
}

// ── User / session auth ───────────────────────────────────────────────────────

func (m *TenantDBManager) UpsertUser(user *models.User) error {
	user.ID = fmt.Sprintf("usr-%d", user.GithubID)
	_, err := m.db.Exec(`
		INSERT INTO users (id, github_id, login, name, email, avatar_url, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(github_id) DO UPDATE SET
			login      = excluded.login,
			name       = excluded.name,
			email      = excluded.email,
			avatar_url = excluded.avatar_url,
			updated_at = CURRENT_TIMESTAMP`,
		user.ID, user.GithubID, user.Login, user.Name, user.Email, user.AvatarURL)
	return err
}

func (m *TenantDBManager) GetSessionUser(sessionID string) (*models.User, error) {
	var u models.User
	err := m.db.QueryRow(`
		SELECT u.id, u.github_id, u.login, u.name, u.email, u.avatar_url
		FROM sessions s
		JOIN users u ON u.id = s.user_id
		WHERE s.id = ? AND s.expires_at > CURRENT_TIMESTAMP`,
		sessionID).Scan(&u.ID, &u.GithubID, &u.Login, &u.Name, &u.Email, &u.AvatarURL)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (m *TenantDBManager) CreateSession(sessionID, userID string) error {
	_, err := m.db.Exec(
		`INSERT INTO sessions (id, user_id, expires_at) VALUES (?, ?, datetime('now', '+7 days'))`,
		sessionID, userID)
	return err
}

func (m *TenantDBManager) DeleteSession(sessionID string) error {
	_, err := m.db.Exec("DELETE FROM sessions WHERE id = ?", sessionID)
	return err
}

// ── API tokens ────────────────────────────────────────────────────────────────

func tokenHash(plain string) string {
	h := sha256.Sum256([]byte(plain))
	return hex.EncodeToString(h[:])
}

func generateAPIToken() string {
	b := make([]byte, 32)
	rand.Read(b)
	return "llmp_" + hex.EncodeToString(b)
}

// CreateAPIToken generates a new bearer token, stores its hash, and returns the plain token.
func (m *TenantDBManager) CreateAPIToken(userID, name string) (string, error) {
	plain := generateAPIToken()
	id := fmt.Sprintf("tok-%d", time.Now().UnixNano())
	_, err := m.db.Exec(
		`INSERT INTO api_tokens (id, user_id, name, token_hash) VALUES (?, ?, ?, ?)`,
		id, userID, name, tokenHash(plain))
	return plain, err
}

// GetUserByToken looks up the user associated with a plain bearer token.
// Updates last_used_at on hit. Returns nil, nil if token not found.
func (m *TenantDBManager) GetUserByToken(plain string) (*models.User, error) {
	hash := tokenHash(plain)
	var u models.User
	err := m.db.QueryRow(`
		SELECT u.id, u.github_id, u.login, u.name, u.email, u.avatar_url
		FROM api_tokens t
		JOIN users u ON u.id = t.user_id
		WHERE t.token_hash = ?`, hash).
		Scan(&u.ID, &u.GithubID, &u.Login, &u.Name, &u.Email, &u.AvatarURL)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	m.db.Exec(`UPDATE api_tokens SET last_used_at = CURRENT_TIMESTAMP WHERE token_hash = ?`, hash)
	return &u, nil
}

// ListAPITokens returns all tokens for a user (name + metadata, never the hash).
func (m *TenantDBManager) ListAPITokens(userID string) ([]models.APIToken, error) {
	rows, err := m.db.Query(
		`SELECT id, name, created_at, last_used_at FROM api_tokens WHERE user_id = ? ORDER BY created_at DESC`,
		userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tokens []models.APIToken
	for rows.Next() {
		var t models.APIToken
		var lastUsed sql.NullTime
		if err := rows.Scan(&t.ID, &t.Name, &t.CreatedAt, &lastUsed); err != nil {
			return nil, err
		}
		if lastUsed.Valid {
			t.LastUsedAt = &lastUsed.Time
		}
		tokens = append(tokens, t)
	}
	return tokens, rows.Err()
}

// DeleteAPIToken removes a token by id, verifying ownership.
func (m *TenantDBManager) DeleteAPIToken(id, userID string) error {
	_, err := m.db.Exec(`DELETE FROM api_tokens WHERE id = ? AND user_id = ?`, id, userID)
	return err
}

// ── Migration from legacy flat files ─────────────────────────────────────────

type scenarioSeed struct {
	ID       string          `json:"id"`
	Keywords []string        `json:"keywords"`
	Response string          `json:"response"`
	ToolName string          `json:"tool_name,omitempty"`
	ToolData json.RawMessage `json:"tool_data,omitempty"`
}

type tenantSeedFile struct {
	State     json.RawMessage `json:"state"`
	Scenarios []scenarioSeed  `json:"scenarios"`
}

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

		raw, err := os.ReadFile(filepath.Join(dataDir, entry.Name()))
		if err != nil {
			log.Printf("[DB] Migration skip %s: %v", entry.Name(), err)
			continue
		}

		// Parse as seed file — if "state" key is present use it; otherwise treat whole file as state (backward compat).
		var seed tenantSeedFile
		json.Unmarshal(raw, &seed)

		stateJSON := raw
		if len(seed.State) > 0 && string(seed.State) != "null" {
			stateJSON = seed.State
		}

		var count int
		m.db.QueryRow("SELECT COUNT(*) FROM tenant_state WHERE tenant_id = ?", tenantID).Scan(&count)
		if count == 0 {
			_, err = m.db.Exec("INSERT OR IGNORE INTO tenant_state (tenant_id, state_json) VALUES (?, ?)", tenantID, string(stateJSON))
			if err != nil {
				log.Printf("[DB] Migration failed %s: %v", tenantID, err)
				continue
			}
			log.Printf("[DB] Migrated tenant: %s", tenantID)
		}

		for _, s := range seed.Scenarios {
			if s.ID == "" {
				continue
			}
			kwJSON, _ := json.Marshal(s.Keywords)
			toolDataJSON := json.RawMessage("{}")
			if len(s.ToolData) > 0 && string(s.ToolData) != "null" {
				toolDataJSON = s.ToolData
			}
			_, err := m.db.Exec(
				`INSERT OR IGNORE INTO tenant_scenarios (id, tenant_id, keywords, response, tool_name, tool_data) VALUES (?, ?, ?, ?, ?, ?)`,
				s.ID, tenantID, string(kwJSON), s.Response, s.ToolName, string(toolDataJSON),
			)
			if err != nil {
				log.Printf("[DB] Scenario seed failed %s/%s: %v", tenantID, s.ID, err)
			}
		}
	}
}
