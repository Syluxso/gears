package kanboard

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// maxAttachmentBytes caps uploads. The file is base64-encoded into a single
// JSON-RPC request, so it is held in memory whole and inflates by about a
// third on the wire; PHP's own upload limits will usually bite before this.
const maxAttachmentBytes = 10 << 20 // 10 MiB

// User is a Kanboard user.
//
// Note: getAllUsers returns a `password` field containing the bcrypt hash.
// It is deliberately not mapped here, so it cannot reach output or logs.
type User struct {
	ID       flexInt `json:"id"`
	Username string  `json:"username"`
	Name     string  `json:"name"`
	Email    string  `json:"email"`
	IsAdmin  flexInt `json:"is_admin"`
}

// Subtask is a checklist item under a task.
type Subtask struct {
	ID            flexInt `json:"id"`
	TaskID        flexInt `json:"task_id"`
	Title         string  `json:"title"`
	Status        flexInt `json:"status"` // 0 todo, 1 in progress, 2 done
	UserID        flexInt `json:"user_id"`
	TimeEstimated float64 `json:"time_estimated"`
	TimeSpent     float64 `json:"time_spent"`
}

// StatusLabel renders the subtask status the way Kanboard names it.
func (s Subtask) StatusLabel() string {
	switch s.Status.Int() {
	case 1:
		return "in progress"
	case 2:
		return "done"
	default:
		return "todo"
	}
}

// LinkType is a relation label such as "blocks" or "relates to".
type LinkType struct {
	ID         flexInt `json:"id"`
	Label      string  `json:"label"`
	OppositeID flexInt `json:"opposite_id"`
}

// TaskLink is a link between two tasks, as returned by getAllTaskLinks.
//
// Careful with `task_id`: when listing links for task A, this field holds the
// id of the task on the *other* end of the relation, not A. The response
// carries no opposite_task_id, so this is the linked task.
type TaskLink struct {
	ID           flexInt `json:"id"` // the task-link id, used to remove it
	LinkedTaskID flexInt `json:"task_id"`
	Label        string  `json:"label"`
	Title        string  `json:"title"`
	ColumnTitle  string  `json:"column_title"`
	ProjectName  string  `json:"project_name"`
	IsActive     flexInt `json:"is_active"`
}

// ExternalTaskLink is a link from a task to a URL.
type ExternalTaskLink struct {
	ID         flexInt `json:"id"`
	TaskID     flexInt `json:"task_id"`
	URL        string  `json:"url"`
	Title      string  `json:"title"`
	Dependency string  `json:"dependency"`
	Type       string  `json:"type"`
}

// TaskFile is an attachment on a task.
type TaskFile struct {
	ID       flexInt `json:"id"`
	Name     string  `json:"name"`
	TaskID   flexInt `json:"task_id"`
	Size     flexInt `json:"size"`
	Date     flexInt `json:"date"`
	UserID   flexInt `json:"user_id"`
	IsImage  flexInt `json:"is_image"`
	Path     string  `json:"path"`
}

// --- Users ---

func (c *Client) GetAllUsers() ([]User, error) {
	var out []User
	err := c.Call("getAllUsers", nil, &out)
	return out, err
}

// ResolveUser accepts a numeric id, a username, or a display name.
func (c *Client) ResolveUser(ref string) (*User, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return nil, fmt.Errorf("no user given")
	}

	users, err := c.GetAllUsers()
	if err != nil {
		return nil, err
	}

	if n, err := strconv.Atoi(ref); err == nil {
		for i := range users {
			if users[i].ID.Int() == n {
				return &users[i], nil
			}
		}
		return nil, fmt.Errorf("no user with id %d", n)
	}

	lower := strings.ToLower(ref)
	for i := range users {
		if strings.ToLower(users[i].Username) == lower || strings.ToLower(users[i].Name) == lower {
			return &users[i], nil
		}
	}

	names := make([]string, 0, len(users))
	for _, u := range users {
		names = append(names, u.Username)
	}
	return nil, fmt.Errorf("no user matching %q\n\n  Available: %s", ref, strings.Join(names, ", "))
}

// --- Subtasks ---

func (c *Client) GetAllSubtasks(taskID int) ([]Subtask, error) {
	var out []Subtask
	err := c.Call("getAllSubtasks", map[string]any{"task_id": taskID}, &out)
	return out, err
}

func (c *Client) CreateSubtask(params map[string]any) (int, error) {
	var id flexInt
	if err := c.Call("createSubtask", params, &id); err != nil {
		return 0, err
	}
	if id.Int() == 0 {
		return 0, fmt.Errorf("Kanboard declined to create the subtask")
	}
	return id.Int(), nil
}

func (c *Client) RemoveSubtask(subtaskID int) error {
	var ok bool
	if err := c.Call("removeSubtask", map[string]any{"subtask_id": subtaskID}, &ok); err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("Kanboard declined to remove subtask %d", subtaskID)
	}
	return nil
}

// --- Internal links ---

func (c *Client) GetAllLinks() ([]LinkType, error) {
	var out []LinkType
	err := c.Call("getAllLinks", nil, &out)
	return out, err
}

// ResolveLinkType accepts a numeric link id or a relation label.
func (c *Client) ResolveLinkType(ref string) (*LinkType, error) {
	links, err := c.GetAllLinks()
	if err != nil {
		return nil, err
	}

	if n, err := strconv.Atoi(strings.TrimSpace(ref)); err == nil {
		for i := range links {
			if links[i].ID.Int() == n {
				return &links[i], nil
			}
		}
		return nil, fmt.Errorf("no link type with id %d", n)
	}

	lower := strings.ToLower(strings.TrimSpace(ref))
	for i := range links {
		if strings.ToLower(links[i].Label) == lower {
			return &links[i], nil
		}
	}

	labels := make([]string, 0, len(links))
	for _, l := range links {
		labels = append(labels, l.Label)
	}
	return nil, fmt.Errorf("no link type %q\n\n  Available: %s", ref, strings.Join(labels, ", "))
}

func (c *Client) GetAllTaskLinks(taskID int) ([]TaskLink, error) {
	var out []TaskLink
	err := c.Call("getAllTaskLinks", map[string]any{"task_id": taskID}, &out)
	return out, err
}

func (c *Client) CreateTaskLink(taskID, oppositeTaskID, linkID int) (int, error) {
	var id flexInt
	if err := c.Call("createTaskLink", map[string]any{
		"task_id": taskID, "opposite_task_id": oppositeTaskID, "link_id": linkID,
	}, &id); err != nil {
		return 0, err
	}
	if id.Int() == 0 {
		return 0, fmt.Errorf("Kanboard declined to create the link")
	}
	return id.Int(), nil
}

func (c *Client) RemoveTaskLink(taskLinkID int) error {
	var ok bool
	if err := c.Call("removeTaskLink", map[string]any{"task_link_id": taskLinkID}, &ok); err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("Kanboard declined to remove link %d", taskLinkID)
	}
	return nil
}

// --- External links ---

func (c *Client) GetAllExternalTaskLinks(taskID int) ([]ExternalTaskLink, error) {
	var out []ExternalTaskLink
	err := c.Call("getAllExternalTaskLinks", map[string]any{"task_id": taskID}, &out)
	return out, err
}

func (c *Client) CreateExternalTaskLink(taskID int, url, title, dependency, linkType string) (int, error) {
	var id flexInt
	if err := c.Call("createExternalTaskLink", map[string]any{
		"task_id": taskID, "url": url, "dependency": dependency,
		"type": linkType, "title": title,
	}, &id); err != nil {
		return 0, err
	}
	if id.Int() == 0 {
		return 0, fmt.Errorf("Kanboard declined to create the external link")
	}
	return id.Int(), nil
}

func (c *Client) RemoveExternalTaskLink(taskID, linkID int) error {
	var ok bool
	if err := c.Call("removeExternalTaskLink",
		map[string]any{"task_id": taskID, "link_id": linkID}, &ok); err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("Kanboard declined to remove external link %d", linkID)
	}
	return nil
}

// --- Attachments ---

func (c *Client) GetAllTaskFiles(taskID int) ([]TaskFile, error) {
	var out []TaskFile
	err := c.Call("getAllTaskFiles", map[string]any{"task_id": taskID}, &out)
	return out, err
}

// CreateTaskFileFromPath reads a local file and uploads it as an attachment.
func (c *Client) CreateTaskFileFromPath(projectID, taskID int, path string) (int, error) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, fmt.Errorf("cannot read %s: %w", path, err)
	}
	if info.IsDir() {
		return 0, fmt.Errorf("%s is a directory, not a file", path)
	}
	if info.Size() > maxAttachmentBytes {
		return 0, fmt.Errorf("%s is %.1f MB; the limit is %d MB",
			path, float64(info.Size())/(1<<20), maxAttachmentBytes>>20)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return 0, fmt.Errorf("cannot read %s: %w", path, err)
	}

	var id flexInt
	if err := c.Call("createTaskFile", map[string]any{
		"project_id": projectID,
		"task_id":    taskID,
		"filename":   filepath.Base(path),
		"blob":       base64.StdEncoding.EncodeToString(data),
	}, &id); err != nil {
		return 0, err
	}
	if id.Int() == 0 {
		return 0, fmt.Errorf("Kanboard declined the upload (check the server's PHP upload limits)")
	}
	return id.Int(), nil
}

func (c *Client) RemoveTaskFile(fileID int) error {
	var ok bool
	if err := c.Call("removeTaskFile", map[string]any{"file_id": fileID}, &ok); err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("Kanboard declined to remove file %d", fileID)
	}
	return nil
}
