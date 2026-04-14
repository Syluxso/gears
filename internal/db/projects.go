package db

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/Syluxso/gears/internal/events"
	"github.com/google/uuid"
)

// ScanAndPopulateProjects scans the projects directory and populates/updates the projects table
func ScanAndPopulateProjects() error {
	if db == nil {
		return fmt.Errorf("database not initialized")
	}

	// Get list of existing projects from database
	existingProjects, err := getAllProjects()
	if err != nil {
		return fmt.Errorf("failed to get existing projects: %w", err)
	}

	// Create map of existing projects by path for quick lookup
	existingMap := make(map[string]*Project)
	for i := range existingProjects {
		existingMap[existingProjects[i].Path] = &existingProjects[i]
	}

	// Scan filesystem for projects
	projectsDir := "projects"
	entries, err := os.ReadDir(projectsDir)
	if err != nil {
		// projects/ doesn't exist - that's okay
		return nil
	}

	foundPaths := make(map[string]bool)

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		projectPath := filepath.Join(projectsDir, entry.Name())
		foundPaths[projectPath] = true

		// Check if project already exists in database
		if existing, exists := existingMap[projectPath]; exists {
			// Project exists - check if it was previously removed
			if existing.Status == "removed" {
				// Restore the project
				if err := restoreProject(existing.ID); err != nil {
					fmt.Fprintf(os.Stderr, "Warning: failed to restore project %s: %v\n", existing.Name, err)
				}
			} else {
				// Update last_scanned_at
				if err := updateProjectScanTime(existing.ID); err != nil {
					fmt.Fprintf(os.Stderr, "Warning: failed to update scan time for %s: %v\n", existing.Name, err)
				}
			}
		} else {
			// New project - add it
			project, err := detectProjectMetadata(projectPath, entry.Name())
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to detect metadata for %s: %v\n", entry.Name(), err)
				continue
			}

			if err := insertProject(project); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to insert project %s: %v\n", project.Name, err)
			}
		}
	}

	// Mark projects as removed if they no longer exist on disk
	for _, existing := range existingProjects {
		if existing.Status != "removed" && !foundPaths[existing.Path] {
			if err := markProjectRemoved(existing.ID, existing.UUID, existing.Name, existing.Path); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to mark project %s as removed: %v\n", existing.Name, err)
			}
		}
	}

	return nil
}

// detectProjectMetadata analyzes a project directory and extracts metadata
func detectProjectMetadata(projectPath, name string) (*Project, error) {
	project := &Project{
		UUID:         uuid.New().String(),
		Name:         name,
		Path:         projectPath,
		Status:       "active",
		WatchEnabled: true,
		CreatedAt:    time.Now(),
	}

	// Detect project type and language
	detectProjectType(project, projectPath)

	// Detect git metadata
	detectGitMetadata(project, projectPath)

	now := time.Now()
	project.LastScannedAt = &now

	return project, nil
}

// detectProjectType determines the project type, language, and framework
func detectProjectType(project *Project, projectPath string) {
	// Check for Go project
	if fileExists(filepath.Join(projectPath, "go.mod")) {
		project.ProjectType = "go"
		project.Language = "Go"
		project.PackageManager = "go mod"
		project.DependencyFile = "go.mod"
		// Could detect framework by parsing go.mod (cobra, gin, etc.)
		project.Framework = detectGoFramework(projectPath)
		return
	}

	// Check for PHP/Laravel project
	if fileExists(filepath.Join(projectPath, "composer.json")) {
		project.ProjectType = "laravel"
		project.Language = "PHP"
		project.PackageManager = "composer"
		project.DependencyFile = "composer.json"
		project.Framework = "Laravel"
		return
	}

	// Check for Node.js project
	if fileExists(filepath.Join(projectPath, "package.json")) {
		project.ProjectType = "node"
		project.Language = "JavaScript"
		project.PackageManager = "npm"
		project.DependencyFile = "package.json"
		project.Framework = detectNodeFramework(projectPath)
		return
	}

	// Check for Python project
	if fileExists(filepath.Join(projectPath, "requirements.txt")) || fileExists(filepath.Join(projectPath, "pyproject.toml")) {
		project.ProjectType = "python"
		project.Language = "Python"
		project.PackageManager = "pip"
		if fileExists(filepath.Join(projectPath, "requirements.txt")) {
			project.DependencyFile = "requirements.txt"
		} else {
			project.DependencyFile = "pyproject.toml"
		}
		return
	}

	// Check for Rust project
	if fileExists(filepath.Join(projectPath, "Cargo.toml")) {
		project.ProjectType = "rust"
		project.Language = "Rust"
		project.PackageManager = "cargo"
		project.DependencyFile = "Cargo.toml"
		return
	}

	// Default to unknown
	project.ProjectType = "unknown"
	project.Language = "Unknown"
}

// detectGoFramework attempts to detect Go framework from go.mod
func detectGoFramework(projectPath string) string {
	goModPath := filepath.Join(projectPath, "go.mod")
	content, err := os.ReadFile(goModPath)
	if err != nil {
		return ""
	}

	contentStr := string(content)
	if strings.Contains(contentStr, "github.com/spf13/cobra") {
		return "Cobra"
	}
	if strings.Contains(contentStr, "github.com/gin-gonic/gin") {
		return "Gin"
	}
	if strings.Contains(contentStr, "github.com/gofiber/fiber") {
		return "Fiber"
	}

	return ""
}

// detectNodeFramework attempts to detect Node.js framework from package.json
func detectNodeFramework(projectPath string) string {
	pkgPath := filepath.Join(projectPath, "package.json")
	content, err := os.ReadFile(pkgPath)
	if err != nil {
		return "Node.js"
	}

	contentStr := string(content)
	if strings.Contains(contentStr, "\"@ionic/angular\"") {
		return "Ionic"
	}
	if strings.Contains(contentStr, "\"react\"") {
		return "React"
	}
	if strings.Contains(contentStr, "\"vue\"") {
		return "Vue"
	}
	if strings.Contains(contentStr, "\"express\"") {
		return "Express"
	}
	if strings.Contains(contentStr, "\"next\"") {
		return "Next.js"
	}

	return "Node.js"
}

// detectGitMetadata extracts git information from a project
func detectGitMetadata(project *Project, projectPath string) {
	gitDir := filepath.Join(projectPath, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		project.HasGit = false
		return
	}

	project.HasGit = true

	// Get remote URL
	cmd := exec.Command("git", "-C", projectPath, "remote", "get-url", "origin")
	if output, err := cmd.Output(); err == nil {
		project.GitRemoteURL = strings.TrimSpace(string(output))
	}

	// Get current branch
	cmd = exec.Command("git", "-C", projectPath, "rev-parse", "--abbrev-ref", "HEAD")
	if output, err := cmd.Output(); err == nil {
		project.GitCurrentBranch = strings.TrimSpace(string(output))
	}

	// Get last commit hash
	cmd = exec.Command("git", "-C", projectPath, "rev-parse", "HEAD")
	if output, err := cmd.Output(); err == nil {
		project.GitLastCommitHash = strings.TrimSpace(string(output))
	}

	// Get last commit date
	cmd = exec.Command("git", "-C", projectPath, "log", "-1", "--format=%cI")
	if output, err := cmd.Output(); err == nil {
		dateStr := strings.TrimSpace(string(output))
		if commitDate, err := time.Parse(time.RFC3339, dateStr); err == nil {
			project.GitLastCommitDate = &commitDate
		}
	}
}

// insertProject adds a new project to the database
func insertProject(project *Project) error {
	query := `
		INSERT INTO projects (
			uuid, name, path, status, has_git, git_remote_url, git_current_branch,
			git_last_commit_hash, git_last_commit_date, project_type, language,
			framework, package_manager, dependency_file, created_at, last_scanned_at,
			watch_enabled
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := db.Exec(
		query,
		project.UUID,
		project.Name,
		project.Path,
		project.Status,
		project.HasGit,
		project.GitRemoteURL,
		project.GitCurrentBranch,
		project.GitLastCommitHash,
		project.GitLastCommitDate,
		project.ProjectType,
		project.Language,
		project.Framework,
		project.PackageManager,
		project.DependencyFile,
		project.CreatedAt,
		project.LastScannedAt,
		project.WatchEnabled,
	)

	if err == nil {
		// Log project_added event
		eventData := events.ProjectData{
			Project:     project.Name,
			Path:        project.Path,
			ProjectType: project.ProjectType,
			Language:    project.Language,
			Framework:   project.Framework,
		}
		_ = events.LogEvent(db, events.EventProjectAdded, events.GetWorkspaceUUID(), project.UUID, eventData)
	}

	return err
}

// updateProjectScanTime updates the last_scanned_at timestamp
func updateProjectScanTime(projectID int64) error {
	query := `UPDATE projects SET last_scanned_at = ? WHERE id = ?`
	_, err := db.Exec(query, time.Now(), projectID)
	return err
}

// markProjectRemoved marks a project as removed
func markProjectRemoved(projectID int64, projectUUID, projectName, projectPath string) error {
	query := `
		UPDATE projects 
		SET status = 'removed', removed_at = ?, watch_enabled = 0 
		WHERE id = ?
	`
	_, err := db.Exec(query, time.Now(), projectID)

	if err == nil {
		// Log project_removed event
		eventData := events.ProjectData{
			Project: projectName,
			Path:    projectPath,
		}
		_ = events.LogEvent(db, events.EventProjectRemoved, events.GetWorkspaceUUID(), projectUUID, eventData)
	}

	return err
}

// restoreProject restores a previously removed project
func restoreProject(projectID int64) error {
	query := `
		UPDATE projects 
		SET status = 'active', removed_at = NULL, watch_enabled = 1 
		WHERE id = ?
	`
	_, err := db.Exec(query, projectID)
	return err
}

// UpdateProjectActivity updates the last_activity_at timestamp for a project by UUID
func UpdateProjectActivity(projectUUID string, activityTime time.Time) error {
	query := `UPDATE projects SET last_activity_at = ? WHERE uuid = ?`
	_, err := db.Exec(query, activityTime, projectUUID)
	return err
}

// UpdateProjectGitState updates cached git state for a project by UUID.
func UpdateProjectGitState(projectUUID, branch, commitHash string, commitDate *time.Time) error {
	query := `
		UPDATE projects
		SET git_current_branch = ?, git_last_commit_hash = ?, git_last_commit_date = ?, last_scanned_at = ?
		WHERE uuid = ?
	`

	_, err := db.Exec(query, branch, commitHash, commitDate, time.Now(), projectUUID)
	return err
}

// getAllProjects retrieves all projects from the database
func getAllProjects() ([]Project, error) {
	query := `
		SELECT id, uuid, name, path, status, has_git, 
		       COALESCE(git_remote_url, ''), COALESCE(git_current_branch, ''),
		       COALESCE(git_last_commit_hash, ''), git_last_commit_date,
		       COALESCE(project_type, ''), COALESCE(language, ''),
		       COALESCE(framework, ''), COALESCE(package_manager, ''),
		       COALESCE(dependency_file, ''), created_at, last_scanned_at,
		       last_activity_at, removed_at, COALESCE(file_count, 0),
		       watch_enabled, COALESCE(description, ''), COALESCE(tags, '')
		FROM projects
	`

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var projects []Project
	for rows.Next() {
		var project Project
		err := rows.Scan(
			&project.ID,
			&project.UUID,
			&project.Name,
			&project.Path,
			&project.Status,
			&project.HasGit,
			&project.GitRemoteURL,
			&project.GitCurrentBranch,
			&project.GitLastCommitHash,
			&project.GitLastCommitDate,
			&project.ProjectType,
			&project.Language,
			&project.Framework,
			&project.PackageManager,
			&project.DependencyFile,
			&project.CreatedAt,
			&project.LastScannedAt,
			&project.LastActivityAt,
			&project.RemovedAt,
			&project.FileCount,
			&project.WatchEnabled,
			&project.Description,
			&project.Tags,
		)
		if err != nil {
			return nil, err
		}
		projects = append(projects, project)
	}

	return projects, nil
}

// GetActiveProjects retrieves all active projects
func GetActiveProjects() ([]Project, error) {
	query := `
		SELECT id, uuid, name, path, status, has_git, 
		       COALESCE(git_remote_url, ''), COALESCE(git_current_branch, ''),
		       COALESCE(git_last_commit_hash, ''), git_last_commit_date,
		       COALESCE(project_type, ''), COALESCE(language, ''),
		       COALESCE(framework, ''), COALESCE(package_manager, ''),
		       COALESCE(dependency_file, ''), created_at, last_scanned_at,
		       last_activity_at, removed_at, COALESCE(file_count, 0),
		       watch_enabled, COALESCE(description, ''), COALESCE(tags, '')
		FROM projects
		WHERE status = 'active'
		ORDER BY last_activity_at DESC NULLS LAST
	`

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var projects []Project
	for rows.Next() {
		var project Project
		err := rows.Scan(
			&project.ID,
			&project.UUID,
			&project.Name,
			&project.Path,
			&project.Status,
			&project.HasGit,
			&project.GitRemoteURL,
			&project.GitCurrentBranch,
			&project.GitLastCommitHash,
			&project.GitLastCommitDate,
			&project.ProjectType,
			&project.Language,
			&project.Framework,
			&project.PackageManager,
			&project.DependencyFile,
			&project.CreatedAt,
			&project.LastScannedAt,
			&project.LastActivityAt,
			&project.RemovedAt,
			&project.FileCount,
			&project.WatchEnabled,
			&project.Description,
			&project.Tags,
		)
		if err != nil {
			return nil, err
		}
		projects = append(projects, project)
	}

	return projects, nil
}

// fileExists checks if a file exists
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
