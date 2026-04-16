package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Syluxso/gears/internal/db"
	"github.com/spf13/cobra"
)

var projectsCmd = &cobra.Command{
	Use:     "projects",
	Aliases: []string{"project"},
	Short:   "List and inspect workspace projects",
	Long:    "Commands for reading project metadata from the projects database table.",
}

var projectsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List active projects from database",
	RunE:  runProjectsList,
}

func init() {
	rootCmd.AddCommand(projectsCmd)
	projectsCmd.AddCommand(projectsListCmd)
}

func runProjectsList(cmd *cobra.Command, args []string) error {
	if _, err := os.Stat(".gears"); os.IsNotExist(err) {
		return fmt.Errorf(".gears directory not found. Run 'gears init' first")
	}

	if err := db.Initialize(); err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}

	if err := db.ScanAndPopulateProjects(); err != nil {
		return fmt.Errorf("failed to refresh project metadata: %w", err)
	}

	projects, err := db.GetActiveProjects()
	if err != nil {
		return fmt.Errorf("failed to load projects: %w", err)
	}

	if len(projects) == 0 {
		fmt.Println("No active projects found in projects table.")
		fmt.Println("Run 'gears init' to scan projects/ and seed metadata.")
		return nil
	}

	fmt.Println("📁 Projects")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	for i, project := range projects {
		stack := strings.TrimSpace(project.Language)
		if stack == "" {
			stack = "Unknown"
		}
		if framework := strings.TrimSpace(project.Framework); framework != "" {
			stack = fmt.Sprintf("%s (%s)", stack, framework)
		}

		fmt.Printf("%d. %s\n", i+1, project.Name)
		fmt.Printf("   Path: %s\n", project.Path)
		fmt.Printf("   Stack: %s\n", stack)
		if project.GitCurrentBranch != "" {
			fmt.Printf("   Git: %s\n", project.GitCurrentBranch)
		}
		fmt.Printf("   Project info: %s\n", projectInfoDocPath(project.Name))
		fmt.Println()
	}

	fmt.Printf("Total: %d project(s)\n", len(projects))
	fmt.Println()
	fmt.Println("Shared architecture summary: .gears/memory/index.md")
	fmt.Println("Workspace overview: .gears/index.md")

	return nil
}

func projectInfoDocPath(projectName string) string {
	slug := strings.TrimSpace(strings.ToLower(projectName))
	slug = strings.ReplaceAll(slug, " ", "-")
	for strings.Contains(slug, "--") {
		slug = strings.ReplaceAll(slug, "--", "-")
	}
	slug = strings.Trim(slug, "-")

	candidate := filepath.Join(".gears", "memory", "projects", slug+".md")
	if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
		return candidate
	}

	return filepath.Join(".gears", "memory", "index.md")
}
