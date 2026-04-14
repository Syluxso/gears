package events

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// Event types
const (
	EventCommand        = "command"
	EventFileChange     = "file_change"
	EventGitCommit      = "git_commit"
	EventGitFetch       = "git_fetch"
	EventProjectAdded   = "project_added"
	EventProjectRemoved = "project_removed"
	EventWatchStart     = "watch_start"
	EventWatchStop      = "watch_stop"
)

// Event represents a workspace event
type Event struct {
	ID          int64
	Timestamp   time.Time
	EventType   string
	ProjectUUID string
	Data        string // JSON blob
	SyncedAt    *time.Time
}

// FileChangeData represents file change event data
type FileChangeData struct {
	Project      string   `json:"project"`
	ProjectUUID  string   `json:"project_uuid"`
	FilesChanged int      `json:"files_changed"`
	SampleFiles  []string `json:"sample_files,omitempty"`
}

// GitCommitData represents git commit event data
type GitCommitData struct {
	CommitHash  string `json:"commit_hash"`
	Message     string `json:"message"`
	Author      string `json:"author"`
	AuthorEmail string `json:"author_email"`
	Timestamp   string `json:"timestamp"`
	Branch      string `json:"branch"`
}

// GitFetchData represents git fetch event data
type GitFetchData struct {
	Remote          string   `json:"remote"`
	BranchesUpdated []string `json:"branches_updated,omitempty"`
	NewCommits      int      `json:"new_commits"`
}

// ProjectData represents project add/remove event data
type ProjectData struct {
	Project     string `json:"project"`
	Path        string `json:"path"`
	ProjectType string `json:"project_type,omitempty"`
	Language    string `json:"language,omitempty"`
	Framework   string `json:"framework,omitempty"`
}

// WatchData represents watch start/stop event data
type WatchData struct {
	SyncEnabled   bool              `json:"sync_enabled,omitempty"`
	Intervals     map[string]string `json:"intervals,omitempty"`
	Reason        string            `json:"reason,omitempty"`
	UptimeSeconds int64             `json:"uptime_seconds,omitempty"`
	EventsLogged  int               `json:"events_logged,omitempty"`
}

// CommandData represents command execution event data
type CommandData struct {
	Command    string `json:"command"`
	Args       string `json:"args,omitempty"`
	ExitCode   int    `json:"exit_code"`
	DurationMS int64  `json:"duration_ms"`
	CWD        string `json:"cwd,omitempty"`
}

// LogEvent logs an event to the database
func LogEvent(db *sql.DB, eventType, projectUUID string, data interface{}) error {
	if db == nil {
		return nil // Silently skip if database not initialized
	}

	// Marshal data to JSON
	var dataJSON string
	if data != nil {
		jsonBytes, err := json.Marshal(data)
		if err != nil {
			return fmt.Errorf("failed to marshal event data: %w", err)
		}
		dataJSON = string(jsonBytes)
	}

	// Insert event
	query := `
		INSERT INTO events (timestamp, event_type, project_uuid, data)
		VALUES (?, ?, ?, ?)
	`

	_, err := db.Exec(query, time.Now(), eventType, projectUUID, dataJSON)
	if err != nil {
		return fmt.Errorf("failed to insert event: %w", err)
	}

	return nil
}

// GetEvents retrieves events with optional filters
func GetEvents(db *sql.DB, limit int, eventType, projectUUID string, since, until *time.Time) ([]Event, error) {
	if db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	query := "SELECT id, timestamp, event_type, project_uuid, data, synced_at FROM events WHERE 1=1"
	args := []interface{}{}

	if eventType != "" {
		query += " AND event_type = ?"
		args = append(args, eventType)
	}

	if projectUUID != "" {
		query += " AND project_uuid = ?"
		args = append(args, projectUUID)
	}

	if since != nil {
		query += " AND timestamp >= ?"
		args = append(args, since)
	}

	if until != nil {
		query += " AND timestamp <= ?"
		args = append(args, until)
	}

	query += " ORDER BY timestamp DESC"

	if limit > 0 {
		query += " LIMIT ?"
		args = append(args, limit)
	}

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query events: %w", err)
	}
	defer rows.Close()

	var events []Event
	for rows.Next() {
		var e Event
		var syncedAt sql.NullTime

		err := rows.Scan(&e.ID, &e.Timestamp, &e.EventType, &e.ProjectUUID, &e.Data, &syncedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan event: %w", err)
		}

		if syncedAt.Valid {
			e.SyncedAt = &syncedAt.Time
		}

		events = append(events, e)
	}

	return events, nil
}

// GetEventStats returns count of events by type
func GetEventStats(db *sql.DB, since *time.Time) (map[string]int, error) {
	if db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	query := "SELECT event_type, COUNT(*) FROM events"
	args := []interface{}{}

	if since != nil {
		query += " WHERE timestamp >= ?"
		args = append(args, since)
	}

	query += " GROUP BY event_type ORDER BY COUNT(*) DESC"

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query stats: %w", err)
	}
	defer rows.Close()

	stats := make(map[string]int)
	for rows.Next() {
		var eventType string
		var count int

		if err := rows.Scan(&eventType, &count); err != nil {
			return nil, fmt.Errorf("failed to scan stats: %w", err)
		}

		stats[eventType] = count
	}

	return stats, nil
}

// GetUnsyncedEvents retrieves events that haven't been synced
func GetUnsyncedEvents(db *sql.DB, limit int) ([]Event, error) {
	if db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	query := `
		SELECT id, timestamp, event_type, project_uuid, data, synced_at 
		FROM events 
		WHERE synced_at IS NULL 
		ORDER BY id ASC
	`

	if limit > 0 {
		query += " LIMIT ?"
	}

	var rows *sql.Rows
	var err error

	if limit > 0 {
		rows, err = db.Query(query, limit)
	} else {
		rows, err = db.Query(query)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to query unsynced events: %w", err)
	}
	defer rows.Close()

	var events []Event
	for rows.Next() {
		var e Event
		var syncedAt sql.NullTime

		err := rows.Scan(&e.ID, &e.Timestamp, &e.EventType, &e.ProjectUUID, &e.Data, &syncedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan event: %w", err)
		}

		if syncedAt.Valid {
			e.SyncedAt = &syncedAt.Time
		}

		events = append(events, e)
	}

	return events, nil
}

// MarkEventsSynced marks events as synced
func MarkEventsSynced(db *sql.DB, eventIDs []int64) error {
	if db == nil {
		return fmt.Errorf("database not initialized")
	}

	if len(eventIDs) == 0 {
		return nil
	}

	// Build placeholders for IN clause
	placeholders := "?"
	for i := 1; i < len(eventIDs); i++ {
		placeholders += ",?"
	}

	query := fmt.Sprintf("UPDATE events SET synced_at = ? WHERE id IN (%s)", placeholders)

	args := []interface{}{time.Now()}
	for _, id := range eventIDs {
		args = append(args, id)
	}

	_, err := db.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("failed to mark events synced: %w", err)
	}

	return nil
}
