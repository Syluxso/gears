package cmd

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/Syluxso/gears/internal/agent"
	"github.com/Syluxso/gears/internal/config"
	"github.com/Syluxso/gears/internal/db"
	"github.com/spf13/cobra"
)

//go:embed templates/.gears
var templateFS embed.FS

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a new .gears directory in the current project",
	Long: `Creates a .gears/ directory structure with all necessary documentation 
templates for AI-assisted development.

Run this command in the root of your project to set up the Gears framework.`,
	RunE: runInit,
}

func init() {
	rootCmd.AddCommand(initCmd)
}

func runInit(cmd *cobra.Command, args []string) error {
	// Check if config already exists
	if config.Exists() {
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("config exists but failed to load: %w", err)
		}

		createdInstructions, err := agent.EnsureCopilotInstructions()
		if err != nil {
			fmt.Printf("Warning: failed to ensure .github/copilot-instructions.md: %v\n", err)
		}

		fmt.Println("✓ .gears already initialized")
		fmt.Printf("✓ Workspace ID: %s\n", cfg.WorkspaceID)
		fmt.Printf("✓ API URL: %s\n", cfg.APIBaseURL)
		if cfg.APIToken != "" {
			fmt.Println("✓ Authenticated: Yes")
		} else {
			fmt.Println("✓ Authenticated: No (run 'gears auth' to authenticate)")
		}
		if createdInstructions {
			fmt.Println("✓ Created .github/copilot-instructions.md with Agent Inbox directive")
		}
		return nil
	}

	// Check if .gears directory exists
	gearsExists := false
	if _, err := os.Stat(".gears"); !os.IsNotExist(err) {
		gearsExists = true
		fmt.Println("Found existing .gears directory, adding .gearbox/config.json...")
	} else {
		fmt.Println("Initializing .gears directory...")
	}

	// Only copy templates if .gears directory doesn't exist
	if !gearsExists {
		// Walk through embedded templates and copy to current directory
		err := fs.WalkDir(templateFS, "templates/.gears", func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}

			// Calculate the relative path
			relPath, err := filepath.Rel("templates", path)
			if err != nil {
				return err
			}

			if d.IsDir() {
				// Create directory
				if err := os.MkdirAll(relPath, 0755); err != nil {
					return fmt.Errorf("failed to create directory %s: %w", relPath, err)
				}
			} else {
				// Read file from embedded FS
				content, err := fs.ReadFile(templateFS, path)
				if err != nil {
					return fmt.Errorf("failed to read template file %s: %w", path, err)
				}

				// Write file to disk
				if err := os.WriteFile(relPath, content, 0644); err != nil {
					return fmt.Errorf("failed to write file %s: %w", relPath, err)
				}

				fmt.Printf("  Created: %s\n", relPath)
			}

			return nil
		})

		if err != nil {
			return fmt.Errorf("failed to initialize .gears: %w", err)
		}
	}

	// Generate and save workspace ID in .gearbox/config.json
	workspaceID := config.GenerateWorkspaceID()
	cfg := &config.Config{
		WorkspaceID: workspaceID,
		APIBaseURL:  config.DefaultAPIBaseURL(),
	}

	if err := cfg.Save(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	// Initialize database
	if err := db.Initialize(); err != nil {
		fmt.Printf("Warning: failed to initialize database: %v\n", err)
	}

	// Scan and populate projects table
	fmt.Println("\nScanning projects directory...")
	if err := db.ScanAndPopulateProjects(); err != nil {
		fmt.Printf("Warning: failed to scan projects: %v\n", err)
	} else {
		// Get active projects to show summary
		projects, err := db.GetActiveProjects()
		if err == nil && len(projects) > 0 {
			fmt.Printf("✓ Found %d project(s):\n", len(projects))
			for _, p := range projects {
				framework := p.Framework
				if framework == "" {
					framework = p.Language
				}
				fmt.Printf("  - %s (%s)\n", p.Name, framework)
			}
		}
	}

	// Close database connection
	_ = db.Close()

	createdInstructions, err := agent.EnsureCopilotInstructions()
	if err != nil {
		fmt.Printf("Warning: failed to ensure .github/copilot-instructions.md: %v\n", err)
	}

	fmt.Println()
	if gearsExists {
		fmt.Println("✓ Added .gearbox/config.json to existing .gears directory!")
	} else {
		fmt.Println("✓ Successfully initialized .gears directory!")
	}
	fmt.Printf("✓ Generated workspace ID: %s\n", workspaceID)
	if createdInstructions {
		fmt.Println("✓ Created .github/copilot-instructions.md with Agent Inbox directive")
	}

	if !gearsExists {
		fmt.Println("\nAgent: FIRST, read .gears/gears-init.md for complete onboarding instructions.")
		fmt.Println("\nThen populate these key files:")
		fmt.Println("  .gears/index.md          → Add project name, description, and project list")
		fmt.Println("  .gears/memory/index.md   → Document tech stack and architecture")
		fmt.Println("  .gears/instructions/index.md → Define coding standards and commands")
		fmt.Println("  .gears/context/index.md  → Set current phase and active work")
	} else {
		fmt.Println("\nNext step: Run 'gears auth' to connect to gears-hub")
	}

	return nil
}
