package workspaceregistry

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

// Entry represents an app-level workspace record used by desktop experiences.
type Entry struct {
	ID           int64
	Name         string
	Path         string
	IsActive     bool
	CreatedAt    time.Time
	LastOpenedAt *time.Time
}

// Registry stores known workspaces in an app-level SQLite database.
type Registry struct {
	db *sql.DB
}

// Open opens or creates the app-level workspace registry database.
func Open() (*Registry, error) {
	dbPath, err := getDBPath()
	if err != nil {
		return nil, err
	}

	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create registry directory: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open workspace registry: %w", err)
	}

	r := &Registry{db: db}
	if err := r.ensureSchema(); err != nil {
		_ = db.Close()
		return nil, err
	}

	return r, nil
}

// Close closes the underlying database handle.
func (r *Registry) Close() error {
	if r == nil || r.db == nil {
		return nil
	}
	return r.db.Close()
}

// OpenWorkspace validates and registers a workspace path, then marks it active.
func (r *Registry) OpenWorkspace(path string) (*Entry, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("workspace registry not initialized")
	}

	workspacePath, err := normalizeWorkspacePath(path)
	if err != nil {
		return nil, err
	}

	if err := validateWorkspacePath(workspacePath); err != nil {
		return nil, err
	}

	now := time.Now()
	name := filepath.Base(workspacePath)

	tx, err := r.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Ensure a single active workspace.
	if _, err := tx.Exec("UPDATE workspaces SET is_active = 0 WHERE is_active = 1"); err != nil {
		return nil, fmt.Errorf("failed to clear active workspace: %w", err)
	}

	res, err := tx.Exec(`
		INSERT INTO workspaces (name, path, is_active, created_at, last_opened_at)
		VALUES (?, ?, 1, ?, ?)
		ON CONFLICT(path) DO UPDATE SET
			name = excluded.name,
			is_active = 1,
			last_opened_at = excluded.last_opened_at
	`, name, workspacePath, now, now)
	if err != nil {
		return nil, fmt.Errorf("failed to upsert workspace: %w", err)
	}

	id, _ := res.LastInsertId()
	if id == 0 {
		if err := tx.QueryRow("SELECT id FROM workspaces WHERE path = ?", workspacePath).Scan(&id); err != nil {
			return nil, fmt.Errorf("failed to load workspace id: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	entry, err := r.GetByID(id)
	if err != nil {
		return nil, err
	}
	return entry, nil
}

// List returns known workspaces with active workspace first.
func (r *Registry) List() ([]Entry, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("workspace registry not initialized")
	}

	rows, err := r.db.Query(`
		SELECT id, name, path, is_active, created_at, last_opened_at
		FROM workspaces
		ORDER BY is_active DESC, COALESCE(last_opened_at, created_at) DESC, name ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to query workspaces: %w", err)
	}
	defer rows.Close()

	var entries []Entry
	for rows.Next() {
		var e Entry
		var lastOpened sql.NullTime
		if err := rows.Scan(&e.ID, &e.Name, &e.Path, &e.IsActive, &e.CreatedAt, &lastOpened); err != nil {
			return nil, fmt.Errorf("failed to scan workspace row: %w", err)
		}
		if lastOpened.Valid {
			e.LastOpenedAt = &lastOpened.Time
		}
		entries = append(entries, e)
	}

	return entries, nil
}

// SetActive marks a workspace active by ID.
func (r *Registry) SetActive(id int64) (*Entry, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("workspace registry not initialized")
	}

	now := time.Now()
	tx, err := r.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec("UPDATE workspaces SET is_active = 0 WHERE is_active = 1"); err != nil {
		return nil, fmt.Errorf("failed to clear active workspace: %w", err)
	}

	res, err := tx.Exec("UPDATE workspaces SET is_active = 1, last_opened_at = ? WHERE id = ?", now, id)
	if err != nil {
		return nil, fmt.Errorf("failed to set active workspace: %w", err)
	}

	affected, _ := res.RowsAffected()
	if affected == 0 {
		return nil, fmt.Errorf("workspace id %d not found", id)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return r.GetByID(id)
}

// Current returns the active workspace if one exists.
func (r *Registry) Current() (*Entry, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("workspace registry not initialized")
	}

	row := r.db.QueryRow(`
		SELECT id, name, path, is_active, created_at, last_opened_at
		FROM workspaces
		WHERE is_active = 1
		LIMIT 1
	`)

	var e Entry
	var lastOpened sql.NullTime
	if err := row.Scan(&e.ID, &e.Name, &e.Path, &e.IsActive, &e.CreatedAt, &lastOpened); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read active workspace: %w", err)
	}

	if lastOpened.Valid {
		e.LastOpenedAt = &lastOpened.Time
	}
	return &e, nil
}

// GetByID returns a single workspace by id.
func (r *Registry) GetByID(id int64) (*Entry, error) {
	row := r.db.QueryRow(`
		SELECT id, name, path, is_active, created_at, last_opened_at
		FROM workspaces
		WHERE id = ?
	`, id)

	var e Entry
	var lastOpened sql.NullTime
	if err := row.Scan(&e.ID, &e.Name, &e.Path, &e.IsActive, &e.CreatedAt, &lastOpened); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("workspace id %d not found", id)
		}
		return nil, fmt.Errorf("failed to read workspace %d: %w", id, err)
	}

	if lastOpened.Valid {
		e.LastOpenedAt = &lastOpened.Time
	}
	return &e, nil
}

func (r *Registry) ensureSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS workspaces (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		path TEXT NOT NULL UNIQUE,
		is_active BOOLEAN NOT NULL DEFAULT 0,
		created_at DATETIME NOT NULL,
		last_opened_at DATETIME
	);

	CREATE INDEX IF NOT EXISTS idx_workspaces_active ON workspaces(is_active);
	CREATE INDEX IF NOT EXISTS idx_workspaces_last_opened ON workspaces(last_opened_at);
	`

	if _, err := r.db.Exec(schema); err != nil {
		return fmt.Errorf("failed to initialize workspace registry schema: %w", err)
	}

	return nil
}

func getDBPath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user config directory: %w", err)
	}
	return filepath.Join(configDir, "gears-desktop", "workspaces.db"), nil
}

func normalizeWorkspacePath(path string) (string, error) {
	if path == "" {
		path = "."
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("failed to resolve workspace path: %w", err)
	}
	return filepath.Clean(abs), nil
}

func validateWorkspacePath(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("workspace path does not exist: %s", path)
	}
	if !info.IsDir() {
		return fmt.Errorf("workspace path is not a directory: %s", path)
	}

	gearsDir := filepath.Join(path, ".gears")
	info, err = os.Stat(gearsDir)
	if err != nil {
		return fmt.Errorf("not a gears workspace (missing .gears): %s", path)
	}
	if !info.IsDir() {
		return fmt.Errorf("invalid gears workspace (.gears is not a directory): %s", path)
	}

	return nil
}
