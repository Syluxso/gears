package cmd

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Syluxso/gears/internal/db"
	"github.com/spf13/cobra"
)

const hydrateFilePath = ".gears/.gearbox/hydrate.md"

type hydrationProject struct {
	Name      string
	Path      string
	Stack     string
	SmokeTest string
	HasGit    bool
}

type hydrationGitState struct {
	Label      string
	Path       string
	Branch     string
	DirtyFiles int
	Err        string
}

type hydrationFileMeta struct {
	Path    string
	ModTime time.Time
}

var (
	hydrateQuick bool
	hydrateFull  bool
	hydrateChat  bool
)

var hydrateCmd = &cobra.Command{
	Use:   "hydrate",
	Short: "Generate a fast onboarding checklist for agents",
	Long: `Outputs a deterministic onboarding checklist that helps an agent quickly
understand how to use Gears and the current project context.

Reads custom instructions from .gears/.gearbox/hydrate.md when available,
and falls back to built-in defaults when missing.`,
	RunE: runHydrate,
}

func init() {
	rootCmd.AddCommand(hydrateCmd)
	hydrateCmd.Flags().BoolVar(&hydrateQuick, "quick", false, "Show a concise hydration checklist")
	hydrateCmd.Flags().BoolVar(&hydrateFull, "full", false, "Show full hydration checklist (default)")
	hydrateCmd.Flags().BoolVar(&hydrateChat, "chat", false, "Run agent chat hydration directives (inbox-style startup check)")
}

func runHydrate(cmd *cobra.Command, args []string) error {
	if hydrateChat {
		if hydrateQuick || hydrateFull {
			return fmt.Errorf("cannot use --chat with --quick or --full")
		}

		if err := db.Initialize(); err != nil {
			return fmt.Errorf("failed to initialize database: %w", err)
		}

		return runInboxRead()
	}

	if hydrateQuick && hydrateFull {
		return fmt.Errorf("cannot use --quick and --full together")
	}

	if _, err := os.Stat(".gears"); os.IsNotExist(err) {
		return fmt.Errorf(".gears directory not found. Run 'gears init' first")
	}

	mode := "full"
	if hydrateQuick {
		mode = "quick"
	}

	steps, source, checklistWarnings := loadHydrationChecklist(mode)

	fmt.Println("🧠 Gears Hydration")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Printf("Mode:   %s\n", mode)
	fmt.Printf("Source: %s\n", source)
	fmt.Println()

	if len(checklistWarnings) > 0 {
		for _, warning := range checklistWarnings {
			fmt.Printf("⚠ %s\n", warning)
		}
		fmt.Println()
	}

	fmt.Println("1) Core Checklist")
	for idx, step := range steps {
		fmt.Printf("   %d. %s\n", idx+1, step)
	}
	fmt.Println()

	projects, projectWarnings := discoverHydrationProjects(mode)
	if len(projectWarnings) > 0 {
		for _, warning := range projectWarnings {
			fmt.Printf("⚠ %s\n", warning)
		}
		fmt.Println()
	}

	fmt.Println("2) Project Tech Snapshot")
	if len(projects) == 0 {
		fmt.Println("   - No project directories found under projects/")
	} else {
		for _, project := range projects {
			gitMark := "no git"
			if project.HasGit {
				gitMark = "git"
			}
			fmt.Printf("   - %s (%s, %s)\n", project.Name, project.Stack, gitMark)
		}
	}
	fmt.Println()

	gitStates := collectGitStates(projects, mode)
	fmt.Println("3) Branch Safety Preflight")
	if len(gitStates) == 0 {
		fmt.Println("   - No git repositories detected in workspace or projects/")
	} else {
		for _, state := range gitStates {
			if state.Err != "" {
				fmt.Printf("   - %s: %s\n", state.Label, state.Err)
				continue
			}
			fmt.Printf("   - %s: branch=%s dirty_files=%d\n", state.Label, state.Branch, state.DirtyFiles)
		}
	}
	fmt.Println()

	fmt.Println("4) Suggested Smoke Tests")
	hasSmoke := false
	for _, project := range projects {
		if project.SmokeTest == "" {
			continue
		}
		hasSmoke = true
		fmt.Printf("   - %s: %s\n", project.Name, project.SmokeTest)
	}
	if !hasSmoke {
		fmt.Println("   - No automatic smoke-test command detected")
	}
	fmt.Println()

	recentLimit := 10
	if mode == "quick" {
		recentLimit = 5
	}

	stories := recentFiles(filepath.Join(".gears", "story"), recentLimit)
	decisions := recentFiles(filepath.Join(".gears", "decisions"), recentLimit)
	sessions := recentFiles(filepath.Join(".gears", "sessions"), recentLimit)

	fmt.Println("5) Recent Context Digest")
	printRecentGroup("stories", stories)
	printRecentGroup("decisions", decisions)
	printRecentGroup("sessions", sessions)
	fmt.Println()

	fmt.Println("6) Guardrails")
	fmt.Println("   - Do not run destructive git commands (reset --hard, checkout --) unless asked")
	fmt.Println("   - Do not edit generated/vendor/build artifacts unless explicitly requested")
	fmt.Println("   - Do not commit or push without user direction")
	fmt.Println()

	reportPath, reportErr := writeHydrationReport(mode, source, steps, projects, gitStates, stories, decisions, sessions)
	if reportErr != nil {
		fmt.Printf("⚠ Could not write hydration report: %v\n", reportErr)
	} else {
		fmt.Printf("✓ Hydration report saved: %s\n", reportPath)
	}

	return nil
}

func loadHydrationChecklist(mode string) ([]string, string, []string) {
	defaults := []string{
		"Run: gears init",
		"Read: .gears/gears-init.md",
		"Run: gears projects list",
		"Read: .gears/memory/index.md",
		"Read: .gears/decisions/index.md",
		"Inspect technologies used in projects/",
		"Review the latest 10 commits for each git project under projects/",
		"Run: gears sessions list",
	}

	if mode == "full" {
		defaults = append(defaults,
			"Run a branch safety preflight across workspace + project repositories",
			"Run one smoke test command per project before code changes",
			"Review latest stories, decisions, and sessions for current priorities",
		)
	}

	content, err := os.ReadFile(hydrateFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return defaults, "built-in defaults", []string{fmt.Sprintf("%s not found; using defaults", hydrateFilePath)}
		}
		return defaults, "built-in defaults", []string{fmt.Sprintf("failed to read %s; using defaults (%v)", hydrateFilePath, err)}
	}

	var custom []string
	scanner := bufio.NewScanner(strings.NewReader(string(content)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") {
			custom = append(custom, strings.TrimSpace(line[2:]))
			continue
		}

		lineWithoutNum := stripLeadingNumber(line)
		if lineWithoutNum != line {
			custom = append(custom, lineWithoutNum)
		}
	}

	if len(custom) == 0 {
		return defaults, "built-in defaults", []string{fmt.Sprintf("%s had no checklist bullets; using defaults", hydrateFilePath)}
	}

	if mode == "quick" && len(custom) > 8 {
		custom = custom[:8]
	}

	return custom, hydrateFilePath, nil
}

func stripLeadingNumber(line string) string {
	i := 0
	for i < len(line) && line[i] >= '0' && line[i] <= '9' {
		i++
	}
	if i == 0 || i >= len(line) || line[i] != '.' {
		return line
	}
	trimmed := strings.TrimSpace(line[i+1:])
	if trimmed == "" {
		return line
	}
	return trimmed
}

func discoverHydrationProjects(mode string) ([]hydrationProject, []string) {
	projectsDir := "projects"
	entries, err := os.ReadDir(projectsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, []string{"projects/ directory not found"}
		}
		return nil, []string{fmt.Sprintf("failed to scan projects/: %v", err)}
	}

	maxProjects := len(entries)
	if mode == "quick" && maxProjects > 8 {
		maxProjects = 8
	}

	projects := make([]hydrationProject, 0, maxProjects)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if len(projects) >= maxProjects {
			break
		}

		projectPath := filepath.Join(projectsDir, entry.Name())
		projects = append(projects, hydrationProject{
			Name:      entry.Name(),
			Path:      projectPath,
			Stack:     detectProjectStack(projectPath),
			SmokeTest: detectSmokeTest(projectPath),
			HasGit:    isGitRepo(projectPath),
		})
	}

	sort.Slice(projects, func(i, j int) bool {
		return projects[i].Name < projects[j].Name
	})

	return projects, nil
}

func detectProjectStack(projectPath string) string {
	if fileExists(filepath.Join(projectPath, "go.mod")) {
		return "Go"
	}

	if fileExists(filepath.Join(projectPath, "composer.json")) {
		return "PHP/Laravel"
	}

	if fileExists(filepath.Join(projectPath, "package.json")) {
		pkgRaw, err := os.ReadFile(filepath.Join(projectPath, "package.json"))
		if err == nil {
			pkg := string(pkgRaw)
			switch {
			case strings.Contains(pkg, "\"next\""):
				return "Node/Next.js"
			case strings.Contains(pkg, "\"@ionic/angular\""):
				return "Node/Ionic"
			case strings.Contains(pkg, "\"react\""):
				return "Node/React"
			case strings.Contains(pkg, "\"vue\""):
				return "Node/Vue"
			}
		}
		return "Node"
	}

	if fileExists(filepath.Join(projectPath, "requirements.txt")) || fileExists(filepath.Join(projectPath, "pyproject.toml")) {
		return "Python"
	}

	if fileExists(filepath.Join(projectPath, "Cargo.toml")) {
		return "Rust"
	}

	return "Unknown"
}

func detectSmokeTest(projectPath string) string {
	if fileExists(filepath.Join(projectPath, "go.mod")) {
		return fmt.Sprintf("(cd %s && go test ./...)", filepath.ToSlash(projectPath))
	}

	if fileExists(filepath.Join(projectPath, "package.json")) {
		pkgRaw, err := os.ReadFile(filepath.Join(projectPath, "package.json"))
		if err != nil {
			return fmt.Sprintf("(cd %s && npm test)", filepath.ToSlash(projectPath))
		}
		pkg := string(pkgRaw)
		switch {
		case strings.Contains(pkg, "\"test\""):
			return fmt.Sprintf("(cd %s && npm test)", filepath.ToSlash(projectPath))
		case strings.Contains(pkg, "\"lint\""):
			return fmt.Sprintf("(cd %s && npm run lint)", filepath.ToSlash(projectPath))
		case strings.Contains(pkg, "\"build\""):
			return fmt.Sprintf("(cd %s && npm run build)", filepath.ToSlash(projectPath))
		default:
			return fmt.Sprintf("(cd %s && npm run test --if-present)", filepath.ToSlash(projectPath))
		}
	}

	if fileExists(filepath.Join(projectPath, "composer.json")) {
		if fileExists(filepath.Join(projectPath, "artisan")) {
			return fmt.Sprintf("(cd %s && php artisan test)", filepath.ToSlash(projectPath))
		}
		if fileExists(filepath.Join(projectPath, "phpunit.xml")) || fileExists(filepath.Join(projectPath, "phpunit.xml.dist")) {
			return fmt.Sprintf("(cd %s && ./vendor/bin/phpunit)", filepath.ToSlash(projectPath))
		}
	}

	if fileExists(filepath.Join(projectPath, "requirements.txt")) || fileExists(filepath.Join(projectPath, "pyproject.toml")) {
		return fmt.Sprintf("(cd %s && python -m pytest -q)", filepath.ToSlash(projectPath))
	}

	return ""
}

func collectGitStates(projects []hydrationProject, mode string) []hydrationGitState {
	states := make([]hydrationGitState, 0, len(projects)+1)

	if isGitRepo(".") {
		states = append(states, readGitState("workspace", "."))
	}

	for _, project := range projects {
		if !project.HasGit {
			continue
		}
		states = append(states, readGitState(project.Name, project.Path))
	}

	if mode == "quick" && len(states) > 8 {
		states = states[:8]
	}

	return states
}

func readGitState(label, path string) hydrationGitState {
	branchCmd := exec.Command("git", "-C", path, "rev-parse", "--abbrev-ref", "HEAD")
	branchOut, err := branchCmd.Output()
	if err != nil {
		return hydrationGitState{Label: label, Path: path, Err: "unable to read git branch"}
	}

	statusCmd := exec.Command("git", "-C", path, "status", "--porcelain")
	statusOut, err := statusCmd.Output()
	if err != nil {
		return hydrationGitState{Label: label, Path: path, Branch: strings.TrimSpace(string(branchOut)), Err: "unable to read git status"}
	}

	statusText := strings.TrimSpace(string(statusOut))
	dirtyCount := 0
	if statusText != "" {
		dirtyCount = len(strings.Split(statusText, "\n"))
	}

	return hydrationGitState{
		Label:      label,
		Path:       path,
		Branch:     strings.TrimSpace(string(branchOut)),
		DirtyFiles: dirtyCount,
	}
}

func isGitRepo(path string) bool {
	cmd := exec.Command("git", "-C", path, "rev-parse", "--is-inside-work-tree")
	if err := cmd.Run(); err != nil {
		return false
	}
	return true
}

func recentFiles(dir string, limit int) []hydrationFileMeta {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	files := make([]hydrationFileMeta, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		fullPath := filepath.Join(dir, entry.Name())
		info, err := entry.Info()
		if err != nil {
			continue
		}

		files = append(files, hydrationFileMeta{
			Path:    fullPath,
			ModTime: info.ModTime(),
		})
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].ModTime.After(files[j].ModTime)
	})

	if limit > 0 && len(files) > limit {
		files = files[:limit]
	}

	return files
}

func printRecentGroup(name string, files []hydrationFileMeta) {
	if len(files) == 0 {
		fmt.Printf("   - %s: none\n", name)
		return
	}

	fmt.Printf("   - %s:\n", name)
	for _, item := range files {
		fmt.Printf("     • %s (modified %s)\n", filepath.ToSlash(item.Path), item.ModTime.Format("2006-01-02 15:04"))
	}
}

func writeHydrationReport(
	mode, source string,
	steps []string,
	projects []hydrationProject,
	gitStates []hydrationGitState,
	stories []hydrationFileMeta,
	decisions []hydrationFileMeta,
	sessions []hydrationFileMeta,
) (string, error) {
	reportDir := filepath.Join(".gears", "sessions")
	if err := os.MkdirAll(reportDir, 0755); err != nil {
		return "", err
	}

	timestamp := time.Now()
	fileName := fmt.Sprintf("hydrate-report-%s.md", timestamp.Format("20060102-150405"))
	reportPath := filepath.Join(reportDir, fileName)

	var b strings.Builder
	b.WriteString("# Hydration Report\n\n")
	b.WriteString(fmt.Sprintf("- Generated: %s\n", timestamp.Format(time.RFC3339)))
	b.WriteString(fmt.Sprintf("- Mode: %s\n", mode))
	b.WriteString(fmt.Sprintf("- Source: %s\n\n", source))

	b.WriteString("## Core Checklist\n")
	for _, step := range steps {
		b.WriteString(fmt.Sprintf("- %s\n", step))
	}
	b.WriteString("\n")

	b.WriteString("## Projects\n")
	if len(projects) == 0 {
		b.WriteString("- None discovered\n")
	} else {
		for _, p := range projects {
			gitMark := "no git"
			if p.HasGit {
				gitMark = "git"
			}
			b.WriteString(fmt.Sprintf("- %s (%s, %s)\n", p.Name, p.Stack, gitMark))
			if p.SmokeTest != "" {
				b.WriteString(fmt.Sprintf("  - Smoke: %s\n", p.SmokeTest))
			}
		}
	}
	b.WriteString("\n")

	b.WriteString("## Git Preflight\n")
	if len(gitStates) == 0 {
		b.WriteString("- No repositories detected\n")
	} else {
		for _, g := range gitStates {
			if g.Err != "" {
				b.WriteString(fmt.Sprintf("- %s: %s\n", g.Label, g.Err))
				continue
			}
			b.WriteString(fmt.Sprintf("- %s: branch=%s dirty_files=%d\n", g.Label, g.Branch, g.DirtyFiles))
		}
	}
	b.WriteString("\n")

	b.WriteString("## Recent Context\n")
	appendRecent(&b, "Stories", stories)
	appendRecent(&b, "Decisions", decisions)
	appendRecent(&b, "Sessions", sessions)
	b.WriteString("\n")

	b.WriteString("## Guardrails\n")
	b.WriteString("- Avoid destructive git commands unless explicitly requested\n")
	b.WriteString("- Avoid editing generated/vendor artifacts unless requested\n")
	b.WriteString("- Avoid commit/push operations without user direction\n")

	if err := os.WriteFile(reportPath, []byte(b.String()), 0644); err != nil {
		return "", err
	}

	return reportPath, nil
}

func appendRecent(b *strings.Builder, name string, files []hydrationFileMeta) {
	b.WriteString(fmt.Sprintf("### %s\n", name))
	if len(files) == 0 {
		b.WriteString("- None\n")
		return
	}

	for _, f := range files {
		b.WriteString(fmt.Sprintf("- %s (%s)\n", filepath.ToSlash(f.Path), f.ModTime.Format("2006-01-02 15:04")))
	}
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
