package kanboard

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Client speaks Kanboard's JSON-RPC 2.0 API. Kanboard exposes a single
// endpoint at /jsonrpc.php with HTTP basic auth.
type Client struct {
	settings *Settings
	endpoint string
	http     *http.Client
}

// New builds a client from resolved settings.
func New(s *Settings) *Client {
	endpoint := strings.TrimRight(s.URL, "/")
	if !strings.HasSuffix(endpoint, "jsonrpc.php") {
		endpoint += "/jsonrpc.php"
	}
	return &Client{
		settings: s,
		endpoint: endpoint,
		http:     &http.Client{Timeout: 30 * time.Second},
	}
}

// NewFromConfig is the common entry point for commands.
func NewFromConfig() (*Client, error) {
	s, err := LoadSettings()
	if err != nil {
		return nil, err
	}
	return New(s), nil
}

// Endpoint returns the resolved JSON-RPC URL (useful for diagnostics).
func (c *Client) Endpoint() string { return c.endpoint }

// BaseURL returns the board's base URL without the JSON-RPC suffix.
func (c *Client) BaseURL() string {
	return strings.TrimSuffix(strings.TrimRight(c.settings.URL, "/"), "/jsonrpc.php")
}

type rpcRequest struct {
	JSONRPC string `json:"jsonrpc"`
	Method  string `json:"method"`
	ID      int    `json:"id"`
	Params  any    `json:"params,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id"`
	Result  json.RawMessage `json:"result"`
	Error   *rpcError       `json:"error"`
}

// Call performs a JSON-RPC call and unmarshals result into out (may be nil).
func (c *Client) Call(method string, params any, out any) error {
	payload, err := json.Marshal(rpcRequest{JSONRPC: "2.0", Method: method, ID: 1, Params: params})
	if err != nil {
		return fmt.Errorf("failed to encode request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, c.endpoint, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("failed to build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(c.settings.Username, c.settings.Token)

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("could not reach Kanboard at %s: %w", c.endpoint, err)
	}
	defer resp.Body.Close()

	var body bytes.Buffer
	if _, err := body.ReadFrom(resp.Body); err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("Kanboard rejected the credentials (HTTP 401)\n\n  Check the username and token:\n    gears kan --set-api-key")
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Kanboard returned HTTP %d: %s", resp.StatusCode,
			c.settings.Redact(truncate(body.String(), 300)))
	}

	var rpcResp rpcResponse
	if err := json.Unmarshal(body.Bytes(), &rpcResp); err != nil {
		return fmt.Errorf("unexpected response from %s (not JSON-RPC): %s",
			c.endpoint, c.settings.Redact(truncate(body.String(), 300)))
	}
	if rpcResp.Error != nil {
		return fmt.Errorf("Kanboard error (%d): %s", rpcResp.Error.Code, rpcResp.Error.Message)
	}
	if out == nil {
		return nil
	}
	if len(rpcResp.Result) == 0 || string(rpcResp.Result) == "null" {
		return nil
	}
	if err := json.Unmarshal(rpcResp.Result, out); err != nil {
		return fmt.Errorf("failed to decode result for %s: %w", method, err)
	}
	return nil
}

func truncate(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// flexInt tolerates Kanboard returning numbers as either JSON numbers or strings.
type flexInt int

func (f *flexInt) UnmarshalJSON(data []byte) error {
	s := strings.Trim(string(data), `"`)
	if s == "" || s == "null" {
		*f = 0
		return nil
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return fmt.Errorf("expected integer, got %s", string(data))
	}
	*f = flexInt(n)
	return nil
}

func (f flexInt) Int() int { return int(f) }

// Project is a Kanboard project.
type Project struct {
	ID         flexInt `json:"id"`
	Name       string  `json:"name"`
	Identifier string  `json:"identifier"`
	IsActive   flexInt `json:"is_active"`
	IsPublic   flexInt `json:"is_public"`
	URL        struct {
		Board string `json:"board"`
		List  string `json:"list"`
	} `json:"url"`
}

// Column is a board column.
type Column struct {
	ID        flexInt `json:"id"`
	Title     string  `json:"title"`
	Position  flexInt `json:"position"`
	ProjectID flexInt `json:"project_id"`
	TaskLimit flexInt `json:"task_limit"`
}

// Task is a Kanboard task (card).
type Task struct {
	ID          flexInt `json:"id"`
	Title       string  `json:"title"`
	Description string  `json:"description"`
	ProjectID   flexInt `json:"project_id"`
	ColumnID    flexInt `json:"column_id"`
	Position    flexInt `json:"position"`
	OwnerID     flexInt `json:"owner_id"`
	IsActive    flexInt `json:"is_active"`
	Reference   string  `json:"reference"`
	Priority    flexInt `json:"priority"`
	ColorID     string  `json:"color_id"`
	SwimlaneID  flexInt `json:"swimlane_id"`
	URL         string  `json:"url"`
}

// Comment is a task comment.
type Comment struct {
	ID       flexInt `json:"id"`
	TaskID   flexInt `json:"task_id"`
	Username string  `json:"username"`
	Name     string  `json:"name"`
	Comment  string  `json:"comment"`
	Date     string  `json:"date_creation"`
}

// --- Reads ---

func (c *Client) GetAllProjects() ([]Project, error) {
	var out []Project
	err := c.Call("getAllProjects", nil, &out)
	return out, err
}

func (c *Client) GetProjectByID(projectID int) (*Project, error) {
	var out Project
	if err := c.Call("getProjectById", map[string]any{"project_id": projectID}, &out); err != nil {
		return nil, err
	}
	if out.ID.Int() == 0 {
		return nil, nil
	}
	return &out, nil
}

func (c *Client) GetColumns(projectID int) ([]Column, error) {
	var out []Column
	err := c.Call("getColumns", map[string]any{"project_id": projectID}, &out)
	return out, err
}

// GetAllTasks returns tasks for a project. statusID 1 = open, 0 = closed.
func (c *Client) GetAllTasks(projectID, statusID int) ([]Task, error) {
	var out []Task
	err := c.Call("getAllTasks", map[string]any{"project_id": projectID, "status_id": statusID}, &out)
	return out, err
}

func (c *Client) GetTask(taskID int) (*Task, error) {
	var out Task
	if err := c.Call("getTask", map[string]any{"task_id": taskID}, &out); err != nil {
		return nil, err
	}
	if out.ID.Int() == 0 {
		return nil, nil
	}
	return &out, nil
}

func (c *Client) GetTaskByReference(projectID int, reference string) (*Task, error) {
	var out Task
	if err := c.Call("getTaskByReference",
		map[string]any{"project_id": projectID, "reference": reference}, &out); err != nil {
		return nil, err
	}
	if out.ID.Int() == 0 {
		return nil, nil
	}
	return &out, nil
}

func (c *Client) SearchTasks(projectID int, query string) ([]Task, error) {
	var out []Task
	err := c.Call("searchTasks", map[string]any{"project_id": projectID, "query": query}, &out)
	return out, err
}

func (c *Client) GetTaskTags(taskID int) (map[string]string, error) {
	out := map[string]string{}
	err := c.Call("getTaskTags", map[string]any{"task_id": taskID}, &out)
	return out, err
}

func (c *Client) GetAllComments(taskID int) ([]Comment, error) {
	var out []Comment
	err := c.Call("getAllComments", map[string]any{"task_id": taskID}, &out)
	return out, err
}

// --- Writes ---

func (c *Client) CreateTask(params map[string]any) (int, error) {
	var id flexInt
	if err := c.Call("createTask", params, &id); err != nil {
		return 0, err
	}
	if id.Int() == 0 {
		return 0, fmt.Errorf("Kanboard declined to create the task (check project_id and column_id)")
	}
	return id.Int(), nil
}

func (c *Client) UpdateTask(params map[string]any) error {
	var ok bool
	if err := c.Call("updateTask", params, &ok); err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("Kanboard declined the update")
	}
	return nil
}

func (c *Client) MoveTaskPosition(projectID, taskID, columnID, position, swimlaneID int) error {
	var ok bool
	if err := c.Call("moveTaskPosition", map[string]any{
		"project_id": projectID, "task_id": taskID,
		"column_id": columnID, "position": position, "swimlane_id": swimlaneID,
	}, &ok); err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("Kanboard declined the move (check the column belongs to this project)")
	}
	return nil
}

func (c *Client) SetTaskTags(projectID, taskID int, tags []string) error {
	var ok bool
	if err := c.Call("setTaskTags", map[string]any{
		"project_id": projectID, "task_id": taskID, "tags": tags,
	}, &ok); err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("Kanboard declined the tag update")
	}
	return nil
}

func (c *Client) CreateComment(taskID, userID int, content string) (int, error) {
	var id flexInt
	if err := c.Call("createComment", map[string]any{
		"task_id": taskID, "user_id": userID, "content": content,
	}, &id); err != nil {
		return 0, err
	}
	return id.Int(), nil
}

func (c *Client) CloseTask(taskID int) error {
	var ok bool
	if err := c.Call("closeTask", map[string]any{"task_id": taskID}, &ok); err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("Kanboard declined to close the task")
	}
	return nil
}

func (c *Client) OpenTask(taskID int) error {
	var ok bool
	if err := c.Call("openTask", map[string]any{"task_id": taskID}, &ok); err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("Kanboard declined to reopen the task")
	}
	return nil
}

func (c *Client) RemoveTask(taskID int) error {
	var ok bool
	if err := c.Call("removeTask", map[string]any{"task_id": taskID}, &ok); err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("Kanboard declined to remove the task")
	}
	return nil
}

// --- Resolution helpers ---

// ResolveProject accepts a numeric id, an identifier (e.g. DXC), or a name.
func (c *Client) ResolveProject(ref string) (*Project, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return nil, fmt.Errorf("no project given")
	}

	if n, err := strconv.Atoi(ref); err == nil {
		p, err := c.GetProjectByID(n)
		if err != nil {
			return nil, err
		}
		if p == nil {
			return nil, fmt.Errorf("no project with id %d\n\n  List them:  gears kan projects", n)
		}
		return p, nil
	}

	projects, err := c.GetAllProjects()
	if err != nil {
		return nil, err
	}
	lower := strings.ToLower(ref)
	for i := range projects {
		if strings.ToLower(projects[i].Identifier) == lower || strings.ToLower(projects[i].Name) == lower {
			return &projects[i], nil
		}
	}
	return nil, fmt.Errorf("no project matching %q\n\n  List them:  gears kan projects", ref)
}

// ResolveColumn accepts a numeric column id or a column title (case-insensitive).
func (c *Client) ResolveColumn(projectID int, ref string) (*Column, error) {
	columns, err := c.GetColumns(projectID)
	if err != nil {
		return nil, err
	}

	if n, err := strconv.Atoi(strings.TrimSpace(ref)); err == nil {
		for i := range columns {
			if columns[i].ID.Int() == n {
				return &columns[i], nil
			}
		}
		return nil, fmt.Errorf("column id %d is not in this project", n)
	}

	lower := strings.ToLower(strings.TrimSpace(ref))
	for i := range columns {
		if strings.ToLower(columns[i].Title) == lower {
			return &columns[i], nil
		}
	}

	titles := make([]string, 0, len(columns))
	for _, col := range columns {
		titles = append(titles, col.Title)
	}
	return nil, fmt.Errorf("no column named %q in this project\n\n  Available: %s",
		ref, strings.Join(titles, ", "))
}

// ResolveTask accepts a numeric task id, or a reference when projectID is non-zero.
func (c *Client) ResolveTask(projectID int, ref string) (*Task, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return nil, fmt.Errorf("no task given")
	}

	if n, err := strconv.Atoi(ref); err == nil {
		t, err := c.GetTask(n)
		if err != nil {
			return nil, err
		}
		if t == nil {
			return nil, fmt.Errorf("no task with id %d", n)
		}
		return t, nil
	}

	if projectID > 0 {
		t, err := c.GetTaskByReference(projectID, ref)
		if err != nil {
			return nil, err
		}
		if t != nil {
			return t, nil
		}
		return nil, fmt.Errorf("no task with reference %q in that project", ref)
	}

	// No project given: search every project for the reference.
	projects, err := c.GetAllProjects()
	if err != nil {
		return nil, err
	}
	for _, p := range projects {
		t, err := c.GetTaskByReference(p.ID.Int(), ref)
		if err != nil {
			continue
		}
		if t != nil {
			return t, nil
		}
	}
	return nil, fmt.Errorf("no task with id or reference %q\n\n  Narrow it down:  gears kan tasks <project>", ref)
}
