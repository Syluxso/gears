package db

import (
	"database/sql"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

// BacklogItem represents an operational work item in the backlog.
// It is the source of truth for status, claiming, ordering, and references.
// A rich description may optionally live in .gears/backlog/backlog--<slug>.md.
type BacklogItem struct {
	ID             int64
	UUID           string
	Label          string
	Slug           string
	Status         string // pending, claimed, in_progress, blocked, completed, cancelled
	SortOrder      int
	ReferenceType  string // e.g. "story", "defect", "chore"
	ReferenceSlug  string
	ClaimedBy      string
	ClaimedAt      *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
	CompletedAt    *time.Time
}

// normalizeSlugForBacklog produces a filesystem / identifier friendly slug.
func normalizeSlugForBacklog(name string) string {
	s := strings.TrimSpace(strings.ToLower(name))
	s = strings.ReplaceAll(s, " ", "-")
	s = regexp.MustCompile(`[^a-z0-9-]+`).ReplaceAllString(s, "-")
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}
	s = strings.Trim(s, "-")
	if s == "" {
		s = "item"
	}
	return s
}

// nextSortOrder returns MAX(sort_order)+1 or 1 if none exist.
func nextSortOrder() (int, error) {
	if db == nil {
		return 0, fmt.Errorf("database not initialized")
	}
	var maxOrder sql.NullInt64
	err := db.QueryRow(`SELECT COALESCE(MAX(sort_order), 0) FROM backlog_items`).Scan(&maxOrder)
	if err != nil {
		return 0, fmt.Errorf("failed to compute next sort_order: %w", err)
	}
	if maxOrder.Valid {
		return int(maxOrder.Int64) + 1, nil
	}
	return 1, nil
}

// CreateBacklogItem inserts a new backlog item.
// sort_order is auto-assigned as MAX+1.
// slug is derived from label if not conflicting (we allow duplicates for now, user can prioritize).
func CreateBacklogItem(label, referenceType, referenceSlug string) (*BacklogItem, error) {
	if db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	label = strings.TrimSpace(label)
	if label == "" {
		return nil, fmt.Errorf("label is required")
	}

	slug := normalizeSlugForBacklog(label)

	order, err := nextSortOrder()
	if err != nil {
		return nil, err
	}

	item := &BacklogItem{
		UUID:          uuid.New().String(),
		Label:         label,
		Slug:          slug,
		Status:        "pending",
		SortOrder:     order,
		ReferenceType: strings.ToLower(strings.TrimSpace(referenceType)),
		ReferenceSlug: strings.TrimSpace(referenceSlug),
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	query := `
		INSERT INTO backlog_items (
			uuid, label, slug, status, sort_order,
			reference_type, reference_slug,
			created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	res, err := db.Exec(
		query,
		item.UUID,
		item.Label,
		item.Slug,
		item.Status,
		item.SortOrder,
		item.ReferenceType,
		item.ReferenceSlug,
		item.CreatedAt,
		item.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to insert backlog item: %w", err)
	}

	id, _ := res.LastInsertId()
	item.ID = id

	return item, nil
}

// GetBacklogItemBySlug returns the item or nil if not found.
func GetBacklogItemBySlug(slug string) (*BacklogItem, error) {
	if db == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	slug = normalizeSlugForBacklog(slug)

	row := db.QueryRow(`
		SELECT id, uuid, label, slug, status, sort_order,
		       COALESCE(reference_type,''), COALESCE(reference_slug,''),
		       COALESCE(claimed_by,''), claimed_at,
		       created_at, updated_at, completed_at
		FROM backlog_items WHERE slug = ?
	`, slug)

	return scanBacklogItem(row)
}

// GetBacklogItemByID returns by numeric id.
func GetBacklogItemByID(id int64) (*BacklogItem, error) {
	if db == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	row := db.QueryRow(`
		SELECT id, uuid, label, slug, status, sort_order,
		       COALESCE(reference_type,''), COALESCE(reference_slug,''),
		       COALESCE(claimed_by,''), claimed_at,
		       created_at, updated_at, completed_at
		FROM backlog_items WHERE id = ?
	`, id)

	return scanBacklogItem(row)
}

// resolveBacklogItem accepts either a numeric string id or a slug.
func resolveBacklogItem(slugOrID string) (*BacklogItem, error) {
	slugOrID = strings.TrimSpace(slugOrID)
	if slugOrID == "" {
		return nil, fmt.Errorf("backlog item identifier required")
	}

	// Try as ID first
	var id int64
	if _, err := fmt.Sscanf(slugOrID, "%d", &id); err == nil && id > 0 {
		item, err := GetBacklogItemByID(id)
		if err == nil && item != nil {
			return item, nil
		}
	}

	// Fall back to slug
	item, err := GetBacklogItemBySlug(slugOrID)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, fmt.Errorf("backlog item not found: %s", slugOrID)
	}
	return item, nil
}

func scanBacklogItem(row *sql.Row) (*BacklogItem, error) {
	var item BacklogItem
	var claimedAt, completedAt sql.NullTime

	err := row.Scan(
		&item.ID,
		&item.UUID,
		&item.Label,
		&item.Slug,
		&item.Status,
		&item.SortOrder,
		&item.ReferenceType,
		&item.ReferenceSlug,
		&item.ClaimedBy,
		&claimedAt,
		&item.CreatedAt,
		&item.UpdatedAt,
		&completedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to scan backlog item: %w", err)
	}

	if claimedAt.Valid {
		item.ClaimedAt = &claimedAt.Time
	}
	if completedAt.Valid {
		item.CompletedAt = &completedAt.Time
	}
	return &item, nil
}

// ListBacklogItems returns items, optionally filtered by status.
// Use statusFilter == "" for all.
// Results are ordered by sort_order ASC (0s first = highest priority), then by id.
func ListBacklogItems(statusFilter string) ([]BacklogItem, error) {
	if db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	query := `
		SELECT id, uuid, label, slug, status, sort_order,
		       COALESCE(reference_type,''), COALESCE(reference_slug,''),
		       COALESCE(claimed_by,''), claimed_at,
		       created_at, updated_at, completed_at
		FROM backlog_items
	`
	args := []interface{}{}

	if statusFilter != "" {
		query += " WHERE status = ?"
		args = append(args, strings.ToLower(statusFilter))
	}
	query += " ORDER BY sort_order ASC, id ASC"

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query backlog items: %w", err)
	}
	defer rows.Close()

	var items []BacklogItem
	for rows.Next() {
		var item BacklogItem
		var claimedAt, completedAt sql.NullTime

		if err := rows.Scan(
			&item.ID, &item.UUID, &item.Label, &item.Slug, &item.Status, &item.SortOrder,
			&item.ReferenceType, &item.ReferenceSlug,
			&item.ClaimedBy, &claimedAt,
			&item.CreatedAt, &item.UpdatedAt, &completedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan backlog row: %w", err)
		}
		if claimedAt.Valid {
			item.ClaimedAt = &claimedAt.Time
		}
		if completedAt.Valid {
			item.CompletedAt = &completedAt.Time
		}
		items = append(items, item)
	}
	return items, nil
}

// ClaimBacklogItem atomically claims the item for the given agent (if not already claimed by someone else).
// Sets status to "claimed".
func ClaimBacklogItem(slugOrID string, agentName string) error {
	if db == nil {
		return fmt.Errorf("database not initialized")
	}
	agentName = strings.TrimSpace(agentName)
	if agentName == "" {
		return fmt.Errorf("agent name is required to claim")
	}

	item, err := resolveBacklogItem(slugOrID)
	if err != nil {
		return err
	}
	if item == nil {
		return fmt.Errorf("backlog item not found")
	}

	if item.Status == "completed" || item.Status == "cancelled" {
		return fmt.Errorf("cannot claim a %s item", item.Status)
	}

	if item.ClaimedBy != "" && item.ClaimedBy != agentName {
		return fmt.Errorf("already claimed by %s", item.ClaimedBy)
	}

	now := time.Now()
	_, err = db.Exec(`
		UPDATE backlog_items
		SET claimed_by = ?, claimed_at = ?, status = 'claimed', updated_at = ?
		WHERE id = ?
	`, agentName, now, now, item.ID)

	if err != nil {
		return fmt.Errorf("failed to claim backlog item: %w", err)
	}
	return nil
}

// CompleteBacklogItem marks the item completed. Returns the item for reference chaining.
func CompleteBacklogItem(slugOrID string) (*BacklogItem, error) {
	if db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	item, err := resolveBacklogItem(slugOrID)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, fmt.Errorf("backlog item not found")
	}

	now := time.Now()
	_, err = db.Exec(`
		UPDATE backlog_items
		SET status = 'completed', completed_at = ?, updated_at = ?
		WHERE id = ?
	`, now, now, item.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to complete backlog item: %w", err)
	}

	// Refresh
	return GetBacklogItemByID(item.ID)
}

// PrioritizeBacklogItem sets sort_order = 0 for the item (top priority group).
// Multiple 0s are allowed and treated as tied for first.
func PrioritizeBacklogItem(slugOrID string) error {
	if db == nil {
		return fmt.Errorf("database not initialized")
	}

	item, err := resolveBacklogItem(slugOrID)
	if err != nil {
		return err
	}
	if item == nil {
		return fmt.Errorf("backlog item not found")
	}

	now := time.Now()
	_, err = db.Exec(`
		UPDATE backlog_items
		SET sort_order = 0, updated_at = ?
		WHERE id = ?
	`, now, item.ID)
	if err != nil {
		return fmt.Errorf("failed to prioritize backlog item: %w", err)
	}
	return nil
}

// GetCurrentClaimed returns the most recently claimed (or in_progress) item for the given agent, if any.
func GetCurrentClaimed(agentName string) (*BacklogItem, error) {
	if db == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	agentName = strings.TrimSpace(agentName)
	if agentName == "" {
		return nil, nil
	}

	row := db.QueryRow(`
		SELECT id, uuid, label, slug, status, sort_order,
		       COALESCE(reference_type,''), COALESCE(reference_slug,''),
		       COALESCE(claimed_by,''), claimed_at,
		       created_at, updated_at, completed_at
		FROM backlog_items
		WHERE claimed_by = ? AND status IN ('claimed', 'in_progress')
		ORDER BY claimed_at DESC, id DESC
		LIMIT 1
	`, agentName)

	return scanBacklogItem(row)
}

// GetBacklogItemByReference finds a backlog item referencing a particular story/etc (best effort, most recent).
func GetBacklogItemByReference(refType, refSlug string) (*BacklogItem, error) {
	if db == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	refType = strings.ToLower(strings.TrimSpace(refType))
	refSlug = strings.TrimSpace(refSlug)

	row := db.QueryRow(`
		SELECT id, uuid, label, slug, status, sort_order,
		       COALESCE(reference_type,''), COALESCE(reference_slug,''),
		       COALESCE(claimed_by,''), claimed_at,
		       created_at, updated_at, completed_at
		FROM backlog_items
		WHERE reference_type = ? AND reference_slug = ?
		ORDER BY id DESC
		LIMIT 1
	`, refType, refSlug)

	return scanBacklogItem(row)
}

// NormalizeForDisplay is a tiny helper for CLI to get a usable slug.
func NormalizeForDisplay(s string) string {
	return normalizeSlugForBacklog(s)
}

// GetBySlugOrID is a convenience for cmd layer.
func GetBySlugOrID(slugOrID string) (*BacklogItem, error) {
	return resolveBacklogItem(slugOrID)
}
