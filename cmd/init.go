package cmd

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

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
	// Check if .gears already exists
	if _, err := os.Stat(".gears"); !os.IsNotExist(err) {
		return fmt.Errorf(".gears directory already exists in the current directory")
	}

	fmt.Println("Initializing .gears directory...")

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

	fmt.Println("\n✓ Successfully initialized .gears directory!")
	fmt.Println("\nAgent: FIRST, read .gears/gears-init.md for complete onboarding instructions.")
	fmt.Println("\nThen populate these key files:")
	fmt.Println("  .gears/index.md          → Add project name, description, and project list")
	fmt.Println("  .gears/memory/index.md   → Document tech stack and architecture")
	fmt.Println("  .gears/instructions/index.md → Define coding standards and commands")
	fmt.Println("  .gears/context/index.md  → Set current phase and active work")

	return nil
}
