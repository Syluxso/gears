package content

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Syluxso/gears/internal/inbox"
	"github.com/google/uuid"
)

const (
	TypeStory = "story"
	TypeADR   = "adr"

	StateMissingFile = "missing_file"
)

type Item struct {
	ID             int64
	UUID           string
	Type           string
	FileType       string
	Label          string
	Slug           string
	State          string
	FilePath       string
	CreatedAt      time.Time
	UpdatedAt      time.Time
	LastSyncedHash string
	LastSyncedAt   *time.Time
}

func NormalizeSlug(name string) string {
	s := strings.TrimSpace(strings.ToLower(name))
	s = strings.ReplaceAll(s, " ", "-")
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}
	s = strings.Trim(s, "-")
	return s
}

func NormalizeState(state string) string {
	s := strings.TrimSpace(strings.ToLower(state))
	s = strings.ReplaceAll(s, " ", "_")
	if s == "" {
		return "created"
	}
	return s
}

func BuildDefaultFilePath(contentType, slug string) (string, error) {
	slug = NormalizeSlug(slug)
	switch contentType {
	case TypeStory:
		return filepath.Join(".gears", "story", "story--"+slug+".md"), nil
	case TypeADR:
		return filepath.Join(".gears", "artifacts", "adr--"+slug+".md"), nil
	default:
		return "", fmt.Errorf("unsupported content type: %s", contentType)
	}
}

func CreateItem(db *sql.DB, contentType, fileType, label, slug, state, filePath string) (*Item, error) {
	if db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	if fileType == "" {
		fileType = "md"
	}

	slug = NormalizeSlug(slug)
	state = NormalizeState(state)

	item := &Item{
		UUID:      uuid.New().String(),
		Type:      contentType,
		FileType:  fileType,
		Label:     strings.TrimSpace(label),
		Slug:      slug,
		State:     state,
		FilePath:  filepath.ToSlash(filePath),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	query := `
		INSERT INTO content_items (
			uuid, type, file_type, label, slug, state, file_path,
			created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	res, err := db.Exec(
		query,
		item.UUID,
		item.Type,
		item.FileType,
		item.Label,
		item.Slug,
		item.State,
		item.FilePath,
		item.CreatedAt,
		item.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create content item: %w", err)
	}

	id, err := res.LastInsertId()
	if err == nil {
		item.ID = id
	}

	return item, nil
}

func UpdateSyncMetadata(db *sql.DB, itemUUID, fileHash string) error {
	if db == nil {
		return fmt.Errorf("database not initialized")
	}

	_, err := db.Exec(
		"UPDATE content_items SET last_synced_hash = ?, last_synced_at = ?, updated_at = ? WHERE uuid = ?",
		fileHash,
		time.Now(),
		time.Now(),
		itemUUID,
	)
	if err != nil {
		return fmt.Errorf("failed to update sync metadata: %w", err)
	}

	return nil
}

func GetByType(db *sql.DB, contentType string) ([]Item, error) {
	if db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	rows, err := db.Query(`
		SELECT id, uuid, type, file_type, label, slug, state, file_path,
		       created_at, updated_at, COALESCE(last_synced_hash, ''), last_synced_at
		FROM content_items
		WHERE type = ?
		ORDER BY created_at ASC, id ASC
	`, contentType)
	if err != nil {
		return nil, fmt.Errorf("failed to query content items: %w", err)
	}
	defer rows.Close()

	items := []Item{}
	for rows.Next() {
		var item Item
		var syncedAt sql.NullTime
		if err := rows.Scan(
			&item.ID, &item.UUID, &item.Type, &item.FileType, &item.Label, &item.Slug, &item.State, &item.FilePath,
			&item.CreatedAt, &item.UpdatedAt, &item.LastSyncedHash, &syncedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan content item: %w", err)
		}
		if syncedAt.Valid {
			item.LastSyncedAt = &syncedAt.Time
		}
		items = append(items, item)
	}

	return items, nil
}

func SyncFromFiles(db *sql.DB) error {
	if db == nil {
		return fmt.Errorf("database not initialized")
	}

	if err := backfillType(db, TypeStory); err != nil {
		return err
	}
	if err := backfillType(db, TypeADR); err != nil {
		return err
	}

	if err := markMissingAndNotify(db); err != nil {
		return err
	}

	return nil
}

func backfillType(db *sql.DB, contentType string) error {
	var dir string
	var prefixes []string
	if contentType == TypeStory {
		dir = filepath.Join(".gears", "story")
		prefixes = []string{"story--", "story-"}
	} else {
		dir = filepath.Join(".gears", "artifacts")
		prefixes = []string{"adr--", "adr-"}
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to read %s directory: %w", contentType, err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if contentType == TypeADR && strings.HasPrefix(name, "adr_example-") {
			continue
		}

		matched := false
		for _, prefix := range prefixes {
			if strings.HasPrefix(name, prefix) {
				matched = true
				break
			}
		}
		if !matched {
			continue
		}

		filePath := filepath.ToSlash(filepath.Join(dir, name))
		existing, err := getByFilePath(db, filePath)
		if err != nil {
			return err
		}

		fileType := strings.TrimPrefix(strings.ToLower(filepath.Ext(name)), ".")
		if fileType == "" {
			fileType = "md"
		}

		label, parsedState, hash := parseFileMetadata(filepath.Join(dir, name), contentType)
		slug := inferSlugFromFilename(name, contentType)

		if existing == nil {
			item, err := CreateItem(db, contentType, fileType, label, slug, parsedState, filePath)
			if err != nil {
				return err
			}
			_ = UpdateSyncMetadata(db, item.UUID, hash)
			continue
		}

		// DB metadata is source of truth. Only auto-heal state when file returns from missing_file.
		if NormalizeState(existing.State) == StateMissingFile {
			healed := parsedState
			if strings.TrimSpace(healed) == "" {
				healed = "created"
			}
			_, _ = db.Exec("UPDATE content_items SET state = ?, updated_at = ? WHERE id = ?", NormalizeState(healed), time.Now(), existing.ID)
		}

		_ = UpdateSyncMetadata(db, existing.UUID, hash)
	}

	return nil
}

func markMissingAndNotify(db *sql.DB) error {
	rows, err := db.Query(`
		SELECT id, uuid, type, label, file_path, state
		FROM content_items
	`)
	if err != nil {
		return fmt.Errorf("failed to query content items for missing-file scan: %w", err)
	}
	defer rows.Close()

	type missingRecord struct {
		id       int64
		itemType string
		label    string
		filePath string
		state    string
	}

	toHandle := []missingRecord{}

	for rows.Next() {
		var id int64
		var itemUUID, itemType, label, filePath, state string
		if err := rows.Scan(&id, &itemUUID, &itemType, &label, &filePath, &state); err != nil {
			return fmt.Errorf("failed to scan content item for missing-file scan: %w", err)
		}
		_ = itemUUID

		if _, err := os.Stat(filepath.FromSlash(filePath)); os.IsNotExist(err) {
			toHandle = append(toHandle, missingRecord{id: id, itemType: itemType, label: label, filePath: filePath, state: state})
		}
	}

	for _, rec := range toHandle {
		if NormalizeState(rec.state) == StateMissingFile {
			continue
		}

		if _, err := db.Exec("UPDATE content_items SET state = ?, updated_at = ? WHERE id = ?", StateMissingFile, time.Now(), rec.id); err != nil {
			return fmt.Errorf("failed to update missing_file state for %s: %w", rec.filePath, err)
		}

		notice := &inbox.Message{
			Level:            inbox.LevelAction,
			Title:            "Missing file for DB metadata",
			Message:          fmt.Sprintf("%s item '%s' is missing file %s. Metadata state set to missing_file.", rec.itemType, rec.label, rec.filePath),
			SuggestedCommand: "gears sync --dry-run",
		}
		if err := inbox.Add(db, notice); err != nil {
			return fmt.Errorf("failed to add inbox notice for missing file %s: %w", rec.filePath, err)
		}
	}

	return nil
}

func getByFilePath(db *sql.DB, filePath string) (*Item, error) {
	row := db.QueryRow(`
		SELECT id, uuid, type, file_type, label, slug, state, file_path,
		       created_at, updated_at, COALESCE(last_synced_hash, ''), last_synced_at
		FROM content_items WHERE file_path = ?
	`, filepath.ToSlash(filePath))

	var item Item
	var syncedAt sql.NullTime
	if err := row.Scan(
		&item.ID, &item.UUID, &item.Type, &item.FileType, &item.Label, &item.Slug, &item.State, &item.FilePath,
		&item.CreatedAt, &item.UpdatedAt, &item.LastSyncedHash, &syncedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to lookup content item by path: %w", err)
	}

	if syncedAt.Valid {
		item.LastSyncedAt = &syncedAt.Time
	}

	return &item, nil
}

func parseFileMetadata(filePath, contentType string) (label, state, fileHash string) {
	contentBytes, err := os.ReadFile(filePath)
	if err != nil {
		return filepath.Base(filePath), "created", ""
	}

	hash := sha256.Sum256(contentBytes)
	fileHash = hex.EncodeToString(hash[:])

	content := string(contentBytes)
	lines := strings.Split(content, "\n")

	label = strings.TrimSuffix(filepath.Base(filePath), filepath.Ext(filePath))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if contentType == TypeStory && strings.HasPrefix(trimmed, "# Story:") {
			label = strings.TrimSpace(strings.TrimPrefix(trimmed, "# Story:"))
		}
		if contentType == TypeADR && strings.HasPrefix(trimmed, "# ") {
			label = strings.TrimSpace(strings.TrimPrefix(trimmed, "# "))
		}
		if strings.HasPrefix(trimmed, "**Status:**") {
			state = strings.TrimSpace(strings.TrimPrefix(trimmed, "**Status:**"))
		}
		if label != "" && state != "" {
			break
		}
	}

	if strings.TrimSpace(state) == "" {
		if contentType == TypeStory {
			state = "pending"
		} else {
			state = "created"
		}
	}

	return label, NormalizeState(state), fileHash
}

func inferSlugFromFilename(filename, contentType string) string {
	base := strings.TrimSuffix(filename, filepath.Ext(filename))
	if contentType == TypeStory {
		if strings.HasPrefix(base, "story--") {
			return NormalizeSlug(strings.TrimPrefix(base, "story--"))
		}
		if strings.HasPrefix(base, "story-") {
			return NormalizeSlug(strings.TrimPrefix(base, "story-"))
		}
	}
	if contentType == TypeADR {
		if strings.HasPrefix(base, "adr--") {
			return NormalizeSlug(strings.TrimPrefix(base, "adr--"))
		}
		if strings.HasPrefix(base, "adr-") {
			return NormalizeSlug(strings.TrimPrefix(base, "adr-"))
		}
	}

	return NormalizeSlug(base)
}
