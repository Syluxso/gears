package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/Syluxso/gears/internal/agent"
	"github.com/Syluxso/gears/internal/config"
	"github.com/Syluxso/gears/internal/db"
	"github.com/spf13/cobra"
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update .gears/ structure (new dirs, templates) and ensure DB schema for current gears version",
	Long:  `Ensures all core .gears/ directories exist and bootstraps missing index.md scaffolding and key templates (e.g. consults/, backlog/) from the latest embedded in this gears binary. Ensures the SQLite DB has all tables (including backlog_items) and runs any migrations. Safe to run repeatedly on existing projects. Never overwrites existing files.`,
	RunE:  runUpdate,
}

func init() {
	rootCmd.AddCommand(updateCmd)
}

func runUpdate(cmd *cobra.Command, args []string) error {
	if _, err := os.Stat(".gears"); os.IsNotExist(err) {
		fmt.Println("No .gears directory found. Run 'gears init' first to initialize.")
		return nil
	}

	fmt.Println("Updating .gears structure to latest...")

	// Ensure .gearbox dir (required for DB + config)
	gearboxDir := filepath.Join(".gears", ".gearbox")
	if err := os.MkdirAll(gearboxDir, 0755); err != nil {
		return fmt.Errorf("failed to ensure .gearbox dir: %w", err)
	}

	// Ensure minimal config.json so agent name, workspace ID etc are available
	// (also creates .gearbox if somehow missed)
	if !config.Exists() {
		workspaceID := config.GenerateWorkspaceID()
		cfg := &config.Config{
			WorkspaceID: workspaceID,
			APIBaseURL:  config.DefaultAPIBaseURL(),
		}
		if err := cfg.Save(); err != nil {
			return fmt.Errorf("failed to create minimal config.json: %w", err)
		}
		fmt.Println("  ✓ Created minimal .gears/.gearbox/config.json")
	} else {
		fmt.Println("  ✓ .gearbox/config.json present")
	}

	// Ensure all core directories exist (new features + standard structure)
	dirsToEnsure := []string{
		filepath.Join(".gears", "artifacts"),
		filepath.Join(".gears", "backlog"),
		filepath.Join(".gears", "consults"),
		filepath.Join(".gears", "context"),
		filepath.Join(".gears", "decisions"),
		filepath.Join(".gears", "instructions"),
		filepath.Join(".gears", "memory"),
		filepath.Join(".gears", "sessions"),
		filepath.Join(".gears", "story"),
	}
	for _, d := range dirsToEnsure {
		if err := os.MkdirAll(d, 0755); err != nil {
			return fmt.Errorf("failed to ensure dir %s: %w", d, err)
		}
	}

	// Copy latest scaffolding templates if missing (from embedded FS in this binary).
	// Only creates; never overwrites existing user content. Covers consults/backlog
	// and other core indexes that may have been added after initial init.
	templates := map[string]string{
		"templates/.gears/index.md":                 filepath.Join(".gears", "index.md"),
		"templates/.gears/gears-init.md":            filepath.Join(".gears", "gears-init.md"),
		"templates/.gears/README.md":                filepath.Join(".gears", "README.md"),
		"templates/.gears/artifacts/index.md":       filepath.Join(".gears", "artifacts", "index.md"),
		"templates/.gears/artifacts/adr_template.md": filepath.Join(".gears", "artifacts", "adr_template.md"),
		"templates/.gears/backlog/index.md":         filepath.Join(".gears", "backlog", "index.md"),
		"templates/.gears/consults/index.md":        filepath.Join(".gears", "consults", "index.md"),
		"templates/.gears/context/index.md":         filepath.Join(".gears", "context", "index.md"),
		"templates/.gears/decisions/index.md":       filepath.Join(".gears", "decisions", "index.md"),
		"templates/.gears/instructions/index.md":    filepath.Join(".gears", "instructions", "index.md"),
		"templates/.gears/memory/index.md":          filepath.Join(".gears", "memory", "index.md"),
		"templates/.gears/sessions/index.md":        filepath.Join(".gears", "sessions", "index.md"),
		"templates/.gears/story/index.md":           filepath.Join(".gears", "story", "index.md"),
	}
	for tmpl, dest := range templates {
		if _, err := os.Stat(dest); os.IsNotExist(err) {
			data, err := templateFS.ReadFile(tmpl)
			if err != nil {
				fmt.Printf("  (no template for %s in this binary, skipping)\n", tmpl)
				continue
			}
			if err := os.WriteFile(dest, data, 0644); err != nil {
				return fmt.Errorf("failed to write %s: %w", dest, err)
			}
			fmt.Printf("  ✓ Created %s\n", dest)
		} else {
			fmt.Printf("  ✓ %s already exists\n", dest)
		}
	}

	// Ensure DB schema (creates tables like backlog_items, content_items, inbox etc. if missing;
	// also runs migrations/backfills for workspace_uuid etc.)
	if err := db.Initialize(); err != nil {
		return fmt.Errorf("failed to initialize DB schema: %w", err)
	}
	fmt.Println("  ✓ DB schema ensured (including backlog_items table)")

	// Refresh the agent instruction files. The gears-owned block is rewritten
	// in place; anything hand-written outside the markers is preserved, which
	// is what makes this safe to run on every update.
	doorways, err := agent.EnsureAgentDoorways()
	if err != nil {
		fmt.Printf("  ⚠ Could not write agent instruction files: %v\n", err)
	} else {
		for _, r := range doorways {
			if r.Action != "unchanged" {
				fmt.Printf("  ✓ %s (%s)\n", r.Path, r.Action)
			}
		}
	}

	fmt.Println("✓ .gears structure and DB updated to match current gears version.")
	fmt.Println("  Run 'gears consult latest' or 'gears backlog list' to verify.")
	fmt.Println("  (New features like consults and backlog + any missing scaffolding are now available.)")

	return nil
}
