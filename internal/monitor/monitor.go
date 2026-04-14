package monitor

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/Syluxso/gears/internal/db"
	"github.com/Syluxso/gears/internal/events"
)

// Config holds monitoring configuration
type Config struct {
	FileCheckInterval   time.Duration
	ProjectScanInterval time.Duration
	GitFetchInterval    time.Duration
	Verbose             bool
}

// DefaultConfig returns default monitoring intervals
func DefaultConfig() Config {
	return Config{
		FileCheckInterval:   30 * time.Second,
		ProjectScanInterval: 5 * time.Minute,
		GitFetchInterval:    8 * time.Minute,
		Verbose:             false,
	}
}

// Monitor manages the workspace monitoring loop
type Monitor struct {
	config             Config
	lastProjectScan    time.Time
	lastGitFetch       time.Time
	stopChan           chan struct{}
	projectChangeCount map[string]int // Track changes per project
}

// New creates a new Monitor instance
func New(config Config) *Monitor {
	return &Monitor{
		config:             config,
		stopChan:           make(chan struct{}),
		projectChangeCount: make(map[string]int),
	}
}

// Start begins the monitoring loop (blocking)
func (m *Monitor) Start() error {
	fmt.Println("")
	fmt.Println("🔍 Monitoring workspace...")
	fmt.Println("   Press Ctrl+C to stop")
	fmt.Println("   File checks: every", m.config.FileCheckInterval)
	fmt.Println("   Project scan: every", m.config.ProjectScanInterval)
	fmt.Println("   Git fetch: every", m.config.GitFetchInterval)
	fmt.Println("")

	// Create tickers for different intervals
	fileTicker := time.NewTicker(m.config.FileCheckInterval)
	defer fileTicker.Stop()

	// Run first checks immediately
	m.checkFileChanges()
	m.lastProjectScan = time.Now()
	m.lastGitFetch = time.Now()

	for {
		select {
		case <-m.stopChan:
			fmt.Println("\n🛑 Stopping watch...")
			return nil

		case <-fileTicker.C:
			m.checkFileChanges()

			// Check if it's time for project scan
			if time.Since(m.lastProjectScan) >= m.config.ProjectScanInterval {
				m.scanProjects()
				m.lastProjectScan = time.Now()
			}

			// Check if it's time for git fetch
			if time.Since(m.lastGitFetch) >= m.config.GitFetchInterval {
				m.fetchGitUpdates()
				m.lastGitFetch = time.Now()
			}
		}
	}
}

// Stop signals the monitor to stop
func (m *Monitor) Stop() {
	close(m.stopChan)
}

// checkFileChanges checks all active projects for file changes using git status
func (m *Monitor) checkFileChanges() {
	projects, err := db.GetActiveProjects()
	if err != nil {
		if m.config.Verbose {
			fmt.Printf("⚠️  Error loading projects: %v\n", err)
		}
		return
	}

	timestamp := time.Now()
	changedProjects := 0

	for _, project := range projects {
		if !project.HasGit {
			continue // Skip non-git projects
		}

		projectPath := project.Path
		// Path is already relative to workspace root (e.g., "projects/gears")
		// or absolute, so use it directly

		// Check if directory exists
		if _, err := os.Stat(projectPath); os.IsNotExist(err) {
			continue // Project directory doesn't exist
		}

		// Run git status --porcelain
		hasChanges, changeCount, err := m.checkGitStatus(projectPath)
		if err != nil {
			if m.config.Verbose {
				fmt.Printf("⚠️  %s: git error: %v\n", project.Name, err)
			}
			continue
		}

		if hasChanges {
			changedProjects++
			m.projectChangeCount[project.Name] = m.projectChangeCount[project.Name] + changeCount

			// Update last_activity_at in database
			if err := db.UpdateProjectActivity(project.UUID, timestamp); err != nil {
				if m.config.Verbose {
					fmt.Printf("⚠️  %s: database update failed: %v\n", project.Name, err)
				}
			}

			// Log file_change event
			eventData := events.FileChangeData{
				Project:      project.Name,
				ProjectUUID:  project.UUID,
				FilesChanged: changeCount,
			}
			_ = events.LogEvent(db.GetDB(), events.EventFileChange, events.GetWorkspaceUUID(), project.UUID, eventData)

			// Show activity indicator
			totalChanges := m.projectChangeCount[project.Name]
			fmt.Printf("📝 %s (%d change%s detected)\n",
				project.Name,
				totalChanges,
				pluralize(totalChanges))
		}

		// Detect local commit history movement even when there are no file changes.
		_ = m.detectAndLogLocalCommits(projectPath, project)
	}

	if changedProjects == 0 && m.config.Verbose {
		fmt.Printf("✓ %s - No changes\n", timestamp.Format("15:04:05"))
	}
}

// checkGitStatus runs git status --porcelain and returns if there are changes
func (m *Monitor) checkGitStatus(projectPath string) (bool, int, error) {
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = projectPath

	output, err := cmd.Output()
	if err != nil {
		return false, 0, err
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	changeCount := 0

	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			changeCount++
		}
	}

	return changeCount > 0, changeCount, nil
}

// scanProjects scans for new/removed projects
func (m *Monitor) scanProjects() {
	fmt.Println("🔄 Scanning for project changes...")

	if err := db.ScanAndPopulateProjects(); err != nil {
		fmt.Printf("⚠️  Project scan failed: %v\n", err)
		return
	}

	projects, _ := db.GetActiveProjects()
	fmt.Printf("✓ Project scan complete (%d active)\n", len(projects))
}

// fetchGitUpdates runs git fetch for all projects
func (m *Monitor) fetchGitUpdates() {
	projects, err := db.GetActiveProjects()
	if err != nil {
		return
	}

	fmt.Println("🌐 Fetching git updates...")
	fetchedCount := 0

	for _, project := range projects {
		if !project.HasGit {
			continue
		}

		projectPath := project.Path
		// Path is already relative to workspace root (e.g., "projects/gears")
		// or absolute, so use it directly

		// Check if directory exists
		if _, err := os.Stat(projectPath); os.IsNotExist(err) {
			continue
		}

		// Run git fetch --all --prune
		cmd := exec.Command("git", "fetch", "--all", "--prune")
		cmd.Dir = projectPath

		if err := cmd.Run(); err != nil {
			if m.config.Verbose {
				fmt.Printf("⚠️  %s: fetch failed: %v\n", project.Name, err)
			}
			continue
		}

		fetchedCount++

		// Detect new commits after fetch
		newCommits := m.detectNewCommits(projectPath, project.Name, project.UUID)

		// Log git_fetch event
		eventData := events.GitFetchData{
			Remote:     "origin",
			NewCommits: newCommits,
		}
		_ = events.LogEvent(db.GetDB(), events.EventGitFetch, events.GetWorkspaceUUID(), project.UUID, eventData)
	}

	if fetchedCount > 0 {
		fmt.Printf("✓ Fetched updates for %d project%s\n", fetchedCount, pluralize(fetchedCount))
	}
}

// pluralize returns "s" if count != 1
func pluralize(count int) string {
	if count == 1 {
		return ""
	}
	return "s"
}

// detectNewCommits detects new commits after git fetch and logs them as events
func (m *Monitor) detectNewCommits(projectPath, projectName, projectUUID string) int {
	// Get current branch
	branchCmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	branchCmd.Dir = projectPath
	branchOutput, err := branchCmd.Output()
	if err != nil {
		return 0
	}
	currentBranch := strings.TrimSpace(string(branchOutput))

	// Check for new commits: HEAD..origin/branch
	remoteBranch := "origin/" + currentBranch
	logCmd := exec.Command("git", "log", "HEAD.."+remoteBranch, "--format=%H|%an|%ae|%s|%ai")
	logCmd.Dir = projectPath
	output, err := logCmd.Output()
	if err != nil {
		return 0
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	commitCount := 0

	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}

		// Parse: hash|author|email|message|timestamp
		parts := strings.SplitN(line, "|", 5)
		if len(parts) != 5 {
			continue
		}

		commitData := events.GitCommitData{
			CommitHash:  parts[0],
			Author:      parts[1],
			AuthorEmail: parts[2],
			Message:     parts[3],
			Timestamp:   parts[4],
			Branch:      currentBranch,
			Source:      "remote",
		}

		// Log each commit as event
		_ = events.LogEvent(db.GetDB(), events.EventGitCommit, events.GetWorkspaceUUID(), projectUUID, commitData)
		commitCount++
	}

	if commitCount > 0 && m.config.Verbose {
		fmt.Printf("   %s: %d new commit%s\n", projectName, commitCount, pluralize(commitCount))
	}

	return commitCount
}

func (m *Monitor) detectAndLogLocalCommits(projectPath string, project db.Project) int {
	currentBranch, currentHash, currentDate, err := getCurrentGitState(projectPath)
	if err != nil || currentHash == "" {
		return 0
	}

	// First observation: set baseline without creating historical backfill noise.
	if strings.TrimSpace(project.GitLastCommitHash) == "" {
		_ = db.UpdateProjectGitState(project.UUID, currentBranch, currentHash, currentDate)
		return 0
	}

	if strings.TrimSpace(project.GitLastCommitHash) == currentHash {
		return 0
	}

	logCmd := exec.Command("git", "log", project.GitLastCommitHash+".."+currentHash, "--format=%H|%an|%ae|%s|%ai")
	logCmd.Dir = projectPath
	output, err := logCmd.Output()
	if err != nil {
		// History may have been rewritten; move baseline forward.
		_ = db.UpdateProjectGitState(project.UUID, currentBranch, currentHash, currentDate)
		return 0
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	commitCount := 0

	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}

		parts := strings.SplitN(line, "|", 5)
		if len(parts) != 5 {
			continue
		}

		commitData := events.GitCommitData{
			CommitHash:  parts[0],
			Author:      parts[1],
			AuthorEmail: parts[2],
			Message:     parts[3],
			Timestamp:   parts[4],
			Branch:      currentBranch,
			Source:      "local",
		}

		_ = events.LogEvent(db.GetDB(), events.EventGitCommit, events.GetWorkspaceUUID(), project.UUID, commitData)
		commitCount++
	}

	_ = db.UpdateProjectGitState(project.UUID, currentBranch, currentHash, currentDate)
	return commitCount
}

func getCurrentGitState(projectPath string) (string, string, *time.Time, error) {
	branchCmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	branchCmd.Dir = projectPath
	branchOutput, err := branchCmd.Output()
	if err != nil {
		return "", "", nil, err
	}

	hashCmd := exec.Command("git", "rev-parse", "HEAD")
	hashCmd.Dir = projectPath
	hashOutput, err := hashCmd.Output()
	if err != nil {
		return "", "", nil, err
	}

	dateCmd := exec.Command("git", "log", "-1", "--format=%cI")
	dateCmd.Dir = projectPath
	dateOutput, err := dateCmd.Output()
	if err != nil {
		return strings.TrimSpace(string(branchOutput)), strings.TrimSpace(string(hashOutput)), nil, nil
	}

	dateStr := strings.TrimSpace(string(dateOutput))
	var commitDate *time.Time
	if dateStr != "" {
		if parsed, err := time.Parse(time.RFC3339, dateStr); err == nil {
			commitDate = &parsed
		}
	}

	return strings.TrimSpace(string(branchOutput)), strings.TrimSpace(string(hashOutput)), commitDate, nil
}
