package inbox

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

const (
	LevelUrgent = "urgent"
	LevelAction = "action"
	LevelInfo   = "info"
)

type Message struct {
	ID               int64
	Level            string
	Title            string
	Message          string
	SuggestedCommand string
	Metadata         string
	IsRead           bool
	ReadAt           *time.Time
	CreatedAt        time.Time
}

func NormalizeLevel(level string) string {
	return strings.ToLower(strings.TrimSpace(level))
}

func IsValidLevel(level string) bool {
	switch NormalizeLevel(level) {
	case LevelUrgent, LevelAction, LevelInfo:
		return true
	default:
		return false
	}
}

func Add(db *sql.DB, m *Message) error {
	if db == nil {
		return fmt.Errorf("database not initialized")
	}

	m.Level = NormalizeLevel(m.Level)
	if !IsValidLevel(m.Level) {
		return fmt.Errorf("invalid level %q (must be urgent, action, or info)", m.Level)
	}

	if strings.TrimSpace(m.Title) == "" {
		return fmt.Errorf("title is required")
	}
	if strings.TrimSpace(m.Message) == "" {
		return fmt.Errorf("message is required")
	}

	query := `
		INSERT INTO inbox (level, title, message, suggested_command, metadata, is_read, created_at)
		VALUES (?, ?, ?, ?, ?, 0, ?)
	`

	res, err := db.Exec(query, m.Level, m.Title, m.Message, m.SuggestedCommand, m.Metadata, time.Now())
	if err != nil {
		return fmt.Errorf("failed to insert inbox message: %w", err)
	}

	id, err := res.LastInsertId()
	if err == nil {
		m.ID = id
	}

	return nil
}

func List(db *sql.DB, unreadOnly bool, limit int) ([]Message, error) {
	if db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	query := `
		SELECT id, level, title, message, COALESCE(suggested_command, ''), COALESCE(metadata, ''),
		       COALESCE(is_read, 0), read_at, created_at
		FROM inbox
	`

	args := []interface{}{}
	if unreadOnly {
		query += " WHERE is_read = 0"
	}

	query += " ORDER BY created_at ASC, id ASC"
	if limit > 0 {
		query += " LIMIT ?"
		args = append(args, limit)
	}

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query inbox: %w", err)
	}
	defer rows.Close()

	msgs := []Message{}
	for rows.Next() {
		var m Message
		var readAt sql.NullTime
		var isRead int

		if err := rows.Scan(&m.ID, &m.Level, &m.Title, &m.Message, &m.SuggestedCommand, &m.Metadata, &isRead, &readAt, &m.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan inbox row: %w", err)
		}

		m.IsRead = isRead != 0
		if readAt.Valid {
			m.ReadAt = &readAt.Time
		}

		msgs = append(msgs, m)
	}

	return msgs, nil
}

// ReadUnread returns unread messages and marks them read atomically.
func ReadUnread(db *sql.DB, limit int) ([]Message, error) {
	if db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	tx, err := db.Begin()
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}

	query := `
		SELECT id, level, title, message, COALESCE(suggested_command, ''), COALESCE(metadata, ''),
		       COALESCE(is_read, 0), read_at, created_at
		FROM inbox
		WHERE is_read = 0
		ORDER BY created_at ASC, id ASC
	`

	args := []interface{}{}
	if limit > 0 {
		query += " LIMIT ?"
		args = append(args, limit)
	}

	rows, err := tx.Query(query, args...)
	if err != nil {
		_ = tx.Rollback()
		return nil, fmt.Errorf("failed to query unread inbox messages: %w", err)
	}

	msgs := []Message{}
	ids := []int64{}
	for rows.Next() {
		var m Message
		var readAt sql.NullTime
		var isRead int

		if err := rows.Scan(&m.ID, &m.Level, &m.Title, &m.Message, &m.SuggestedCommand, &m.Metadata, &isRead, &readAt, &m.CreatedAt); err != nil {
			rows.Close()
			_ = tx.Rollback()
			return nil, fmt.Errorf("failed to scan unread inbox row: %w", err)
		}

		m.IsRead = isRead != 0
		if readAt.Valid {
			m.ReadAt = &readAt.Time
		}

		msgs = append(msgs, m)
		ids = append(ids, m.ID)
	}
	rows.Close()

	if len(ids) > 0 {
		placeholders := "?"
		updateArgs := []interface{}{time.Now(), ids[0]}
		for i := 1; i < len(ids); i++ {
			placeholders += ",?"
			updateArgs = append(updateArgs, ids[i])
		}

		updateQuery := fmt.Sprintf("UPDATE inbox SET is_read = 1, read_at = ? WHERE id IN (%s)", placeholders)
		if _, err := tx.Exec(updateQuery, updateArgs...); err != nil {
			_ = tx.Rollback()
			return nil, fmt.Errorf("failed to mark inbox messages read: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit inbox read transaction: %w", err)
	}

	return msgs, nil
}

func ClearUnread(db *sql.DB) (int64, error) {
	if db == nil {
		return 0, fmt.Errorf("database not initialized")
	}

	res, err := db.Exec("UPDATE inbox SET is_read = 1, read_at = ? WHERE is_read = 0", time.Now())
	if err != nil {
		return 0, fmt.Errorf("failed to clear inbox: %w", err)
	}

	count, err := res.RowsAffected()
	if err != nil {
		return 0, nil
	}

	return count, nil
}
