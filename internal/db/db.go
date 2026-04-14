package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

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
		project_uuid TEXT,
		data TEXT,
		synced_at DATETIME
	);

	CREATE INDEX IF NOT EXISTS idx_events_type ON events(event_type);
	CREATE INDEX IF NOT EXISTS idx_events_timestamp ON events(timestamp);
	CREATE INDEX IF NOT EXISTS idx_events_project ON events(project_uuid);
	CREATE INDEX IF NOT EXISTS idx_events_synced ON events(synced_at);
	`

	if _, err := db.Exec(schema); err != nil {
		return fmt.Errorf("failed to create schema: %w", err)
	}

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
