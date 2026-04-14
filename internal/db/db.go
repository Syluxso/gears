package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Syluxso/gears/internal/config"
	_ "modernc.org/sqlite"
)

const (
	DBFileName = ".gears/.gearbox/gears.db"
)

var (
	db *sql.DB
)

// CommandLog represents a single command execution
type CommandLog struct {
	ID           int64
	Command      string
	Args         string // JSON array
	ExitCode     int
	Timestamp    time.Time
	DurationMS   int64
	CWD          string
	WorkspaceID  string
	ErrorMessage string
}

// Project represents a project in the workspace
type Project struct {
	ID                int64
	UUID              string
	Name              string
	Path              string
	Status            string // "active", "removed", "archived", "paused"
	HasGit            bool
	GitRemoteURL      string
	GitCurrentBranch  string
	GitLastCommitHash string
	GitLastCommitDate *time.Time
	ProjectType       string
	Language          string
	Framework         string
	PackageManager    string
	DependencyFile    string
	CreatedAt         time.Time
	LastScannedAt     *time.Time
	LastActivityAt    *time.Time
	RemovedAt         *time.Time
	FileCount         int
	WatchEnabled      bool
	Description       string
	Tags              string // JSON array
}

// Initialize opens or creates the database and sets up schema
func Initialize() error {
	// Check if .gears/.gearbox exists
	dbDir := filepath.Dir(DBFileName)
	if _, err := os.Stat(dbDir); os.IsNotExist(err) {
		// Database directory doesn't exist, skip initialization
		// This is expected when running outside a .gears workspace
		return nil
	}

	var err error
	db, err = sql.Open("sqlite", DBFileName)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}

	// Create schema if it doesn't exist
	schema := `
	CREATE TABLE IF NOT EXISTS command_log (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		command TEXT NOT NULL,
		args TEXT,
		exit_code INTEGER,
		timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
		duration_ms INTEGER,
		cwd TEXT,
		workspace_id TEXT,
		error_message TEXT
	);

	CREATE INDEX IF NOT EXISTS idx_timestamp ON command_log(timestamp);
	CREATE INDEX IF NOT EXISTS idx_command ON command_log(command);

	CREATE TABLE IF NOT EXISTS projects (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		uuid TEXT UNIQUE NOT NULL,
		name TEXT NOT NULL,
		path TEXT NOT NULL,
		status TEXT DEFAULT 'active',
		has_git BOOLEAN DEFAULT 0,
		git_remote_url TEXT,
		git_current_branch TEXT,
		git_last_commit_hash TEXT,
		git_last_commit_date DATETIME,
		project_type TEXT,
		language TEXT,
		framework TEXT,
		package_manager TEXT,
		dependency_file TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		last_scanned_at DATETIME,
		last_activity_at DATETIME,
		removed_at DATETIME,
		file_count INTEGER,
		watch_enabled BOOLEAN DEFAULT 1,
		description TEXT,
		tags TEXT
	);

	CREATE INDEX IF NOT EXISTS idx_projects_name ON projects(name);
	CREATE INDEX IF NOT EXISTS idx_projects_type ON projects(project_type);
	CREATE INDEX IF NOT EXISTS idx_projects_status ON projects(status);
	CREATE INDEX IF NOT EXISTS idx_projects_activity ON projects(last_activity_at);

	CREATE TABLE IF NOT EXISTS events (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
		event_type TEXT NOT NULL,
		workspace_uuid TEXT,
		project_uuid TEXT,
		data TEXT,
		synced_at DATETIME
	);

	CREATE INDEX IF NOT EXISTS idx_events_type ON events(event_type);
	CREATE INDEX IF NOT EXISTS idx_events_timestamp ON events(timestamp);
	CREATE INDEX IF NOT EXISTS idx_events_project ON events(project_uuid);
	CREATE INDEX IF NOT EXISTS idx_events_synced ON events(synced_at);

	CREATE TABLE IF NOT EXISTS inbox (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		level TEXT NOT NULL,
		title TEXT NOT NULL,
		message TEXT NOT NULL,
		suggested_command TEXT,
		metadata TEXT,
		is_read BOOLEAN DEFAULT 0,
		read_at DATETIME,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_inbox_is_read ON inbox(is_read);
	CREATE INDEX IF NOT EXISTS idx_inbox_level ON inbox(level);
	CREATE INDEX IF NOT EXISTS idx_inbox_created_at ON inbox(created_at);

	CREATE TABLE IF NOT EXISTS content_items (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		uuid TEXT UNIQUE NOT NULL,
		type TEXT NOT NULL,
		file_type TEXT NOT NULL DEFAULT 'md',
		label TEXT NOT NULL,
		slug TEXT NOT NULL,
		state TEXT NOT NULL,
		file_path TEXT UNIQUE NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		last_synced_hash TEXT,
		last_synced_at DATETIME
	);

	CREATE INDEX IF NOT EXISTS idx_content_items_type ON content_items(type);
	CREATE INDEX IF NOT EXISTS idx_content_items_state ON content_items(state);
	CREATE INDEX IF NOT EXISTS idx_content_items_slug ON content_items(slug);
	CREATE INDEX IF NOT EXISTS idx_content_items_file_path ON content_items(file_path);
	`

	if _, err := db.Exec(schema); err != nil {
		return fmt.Errorf("failed to create schema: %w", err)
	}

	// Migrate older databases that predate workspace_uuid on events table.
	if err := ensureEventsWorkspaceUUIDColumn(); err != nil {
		return fmt.Errorf("failed to migrate events table: %w", err)
	}

	if err := ensureInboxLevelConstraint(); err != nil {
		return fmt.Errorf("failed to migrate inbox table: %w", err)
	}

	return nil
}

func ensureEventsWorkspaceUUIDColumn() error {
	if db == nil {
		return nil
	}

	rows, err := db.Query("PRAGMA table_info(events)")
	if err != nil {
		return fmt.Errorf("failed to inspect events table: %w", err)
	}
	defer rows.Close()

	hasWorkspaceUUID := false
	for rows.Next() {
		var cid int
		var name, colType string
		var notNull int
		var dflt sql.NullString
		var pk int

		if err := rows.Scan(&cid, &name, &colType, &notNull, &dflt, &pk); err != nil {
			return fmt.Errorf("failed to scan events table info: %w", err)
		}

		if name == "workspace_uuid" {
			hasWorkspaceUUID = true
			break
		}
	}

	if !hasWorkspaceUUID {
		if _, err := db.Exec("ALTER TABLE events ADD COLUMN workspace_uuid TEXT"); err != nil {
			return fmt.Errorf("failed to add workspace_uuid column: %w", err)
		}
	}

	if _, err := db.Exec("CREATE INDEX IF NOT EXISTS idx_events_workspace ON events(workspace_uuid)"); err != nil {
		return fmt.Errorf("failed to create events workspace index: %w", err)
	}

	// Backfill old rows created before workspace_uuid existed.
	if cfg, err := config.Load(); err == nil && cfg.WorkspaceID != "" {
		if _, err := db.Exec(
			"UPDATE events SET workspace_uuid = ? WHERE workspace_uuid IS NULL OR workspace_uuid = ''",
			cfg.WorkspaceID,
		); err != nil {
			// Best-effort only. Another running process may temporarily lock the DB.
			// New events will still include workspace_uuid.
		}
	}

	return nil
}

func ensureInboxLevelConstraint() error {
	if db == nil {
		return nil
	}

	// Best-effort normalization for older rows.
	_, _ = db.Exec("UPDATE inbox SET level = LOWER(level) WHERE level IS NOT NULL")

	// Backfill empty/NULL levels to info to avoid invalid records.
	_, _ = db.Exec("UPDATE inbox SET level = 'info' WHERE level IS NULL OR level = ''")

	return nil
}

// Close closes the database connection
func Close() error {
	if db != nil {
		return db.Close()
	}
	return nil
}

// GetDB returns the database connection
func GetDB() *sql.DB {
	return db
}

// LogCommand records a command execution
func LogCommand(log *CommandLog) error {
	// Skip if database isn't initialized
	if db == nil {
		return nil
	}

	query := `
		INSERT INTO command_log (
			command, args, exit_code, timestamp, duration_ms, 
			cwd, workspace_id, error_message
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := db.Exec(
		query,
		log.Command,
		log.Args,
		log.ExitCode,
		log.Timestamp,
		log.DurationMS,
		log.CWD,
		log.WorkspaceID,
		log.ErrorMessage,
	)

	if err != nil {
		// Don't fail the command if logging fails
		// Just return the error for optional logging
		return fmt.Errorf("failed to log command: %w", err)
	}

	return nil
}

// GetRecentCommands retrieves the most recent command logs
func GetRecentCommands(limit int) ([]CommandLog, error) {
	if db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	query := `
		SELECT id, command, args, exit_code, timestamp, duration_ms, 
		       cwd, workspace_id, COALESCE(error_message, '') as error_message
		FROM command_log
		ORDER BY timestamp DESC
		LIMIT ?
	`

	rows, err := db.Query(query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query commands: %w", err)
	}
	defer rows.Close()

	var logs []CommandLog
	for rows.Next() {
		var log CommandLog
		err := rows.Scan(
			&log.ID,
			&log.Command,
			&log.Args,
			&log.ExitCode,
			&log.Timestamp,
			&log.DurationMS,
			&log.CWD,
			&log.WorkspaceID,
			&log.ErrorMessage,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}
		logs = append(logs, log)
	}

	// Reverse slice to display oldest-first (ASC order)
	for i, j := 0, len(logs)-1; i < j; i, j = i+1, j-1 {
		logs[i], logs[j] = logs[j], logs[i]
	}

	return logs, nil
}

// ArgsToJSON converts command args to JSON array string
func ArgsToJSON(args []string) string {
	if len(args) == 0 {
		return "[]"
	}
	data, err := json.Marshal(args)
	if err != nil {
		return "[]"
	}
	return string(data)
}
