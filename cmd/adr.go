package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Syluxso/gears/internal/content"
	"github.com/Syluxso/gears/internal/db"
	"github.com/spf13/cobra"
)

var adrCmd = &cobra.Command{
	Use:   "adr",
	Short: "Manage Architectural Decision Records (working code documentation)",
	Long:  `Create and list ADR files in .gears/artifacts/ that document working code patterns.`,
}

var adrNewCmd = &cobra.Command{
	Use:   "new <name>",
	Short: "Create a new ADR file with template",
	Long: `Creates a new ADR file in .gears/artifacts/ using the format adr--<name>.md.

ADRs document WORKING code patterns extracted from real implementations, not theoretical designs.`,
	Args: cobra.ExactArgs(1),
	RunE: runAdrNew,
}

var adrListCmd = &cobra.Command{
	Use:   "list",
	Short: "Show all ADRs in the project",
	Long:  `Lists all ADR files in .gears/artifacts/.`,
	RunE:  runAdrList,
}

func init() {
	rootCmd.AddCommand(adrCmd)
	adrCmd.AddCommand(adrNewCmd)
	adrCmd.AddCommand(adrListCmd)
}

func runAdrNew(cmd *cobra.Command, args []string) error {
	// Check if .gears exists
	if _, err := os.Stat(".gears"); os.IsNotExist(err) {
		return fmt.Errorf(".gears directory not found. Run 'gears init' first")
	}

	// Check if artifacts directory exists
	artifactsDir := filepath.Join(".gears", "artifacts")
	if _, err := os.Stat(artifactsDir); os.IsNotExist(err) {
		return fmt.Errorf(".gears/artifacts directory not found")
	}

	// Get ADR name and create filename
	name := args[0]
	slug := content.NormalizeSlug(name)
	adrFile, err := content.BuildDefaultFilePath(content.TypeADR, slug)
	if err != nil {
		return err
	}

	if err := db.Initialize(); err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}

	item, err := content.CreateItem(db.GetDB(), content.TypeADR, "md", name, slug, "created", adrFile)
	if err != nil {
		return fmt.Errorf("failed to create ADR metadata: %w", err)
	}

	// Check if file already exists
	if _, err := os.Stat(adrFile); err == nil {
		return fmt.Errorf("ADR file already exists: %s", adrFile)
	}

	// Create ADR file with template
	today := time.Now().Format("2006-01-02")
	adrTemplate := fmt.Sprintf(`# %s

**Documented:** %s
**Status:** Draft

## Overview

_[What is this system/pattern? What problem does it solve?]_

**Location**: _[file path, package, or directory]_
**Package/Version**: _[if using external package]_

## Key Features

### 1. _[Feature Name]_

_[Description of the feature]_

#### Implementation

`+"```"+`[language]
// Code example showing the implementation
`+"```"+`

### 2. _[Feature Name]_

_[Description of the feature]_

## Database Structure

_[If applicable - tables, columns, relationships]_

**Tables**:
- `+"`"+`table_name`+"`"+` - Description
  - `+"`"+`column_name`+"`"+` - Type - Description

## Usage Patterns

_[How other code uses this system. Common patterns and best practices.]_

#### Example Usage

`+"```"+`[language]
// Code example showing usage
`+"```"+`

## Key Decisions

_[Important architectural or implementation choices and why they were made]_

## References

- Related ADR: _[link if applicable]_
- Documentation: _[external docs if applicable]_
`, name, today)

	if err := os.WriteFile(adrFile, []byte(adrTemplate), 0644); err != nil {
		_ = content.SyncFromFiles(db.GetDB())
		return fmt.Errorf("failed to create ADR file: %w", err)
	}

	_ = content.SyncFromFiles(db.GetDB())
	_ = content.UpdateSyncMetadata(db.GetDB(), item.UUID, "")

	fmt.Printf("✓ Created ADR: %s\n\n", adrFile)
	fmt.Println("Agent next: Document a WORKING system by interviewing the user.")
	fmt.Println()
	fmt.Println("  1. Ask user: \"Which system or pattern should I document?\"")
	fmt.Println("  2. Review the actual implementation code (files, classes, database)")
	fmt.Println("  3. Extract and organize:")
	fmt.Println("     • Overview (what it is, where it lives, key package/version)")
	fmt.Println("     • Key Features (broken down by major capabilities)")
	fmt.Println("     • Implementation Details (actual code snippets)")
	fmt.Println("     • Database Structure (if applicable - tables, relationships)")
	fmt.Println("     • Usage Patterns (how other code uses this system)")
	fmt.Println()
	fmt.Println("  4. Follow the structure pattern from example ADRs:")
	fmt.Println("     • .gears/artifacts/adr_example-service-layer.md")
	fmt.Println("     • .gears/artifacts/adr_example-permissions.md")
	fmt.Println()
	fmt.Println("  5. CRITICAL: Document ONLY what currently exists, not future plans.")
	fmt.Println()
	fmt.Println("This is an architectural export from proven, working code.")

	return nil
}

func runAdrList(cmd *cobra.Command, args []string) error {
	// Check if .gears exists
	if _, err := os.Stat(".gears"); os.IsNotExist(err) {
		return fmt.Errorf(".gears directory not found. Run 'gears init' first")
	}

	artifactsDir := filepath.Join(".gears", "artifacts")
	if _, err := os.Stat(artifactsDir); os.IsNotExist(err) {
		return fmt.Errorf(".gears/artifacts directory not found")
	}

	if err := db.Initialize(); err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}

	if err := content.SyncFromFiles(db.GetDB()); err != nil {
		return fmt.Errorf("failed to sync ADRs: %w", err)
	}

	items, err := content.GetByType(db.GetDB(), content.TypeADR)
	if err != nil {
		return fmt.Errorf("failed to query ADRs: %w", err)
	}

	var adrFiles []string
	var exampleFiles []string

	entries, err := os.ReadDir(artifactsDir)
	if err == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			if strings.HasPrefix(entry.Name(), "adr_example-") {
				exampleFiles = append(exampleFiles, entry.Name())
			}
		}
	}

	for _, item := range items {
		adrFiles = append(adrFiles, filepath.Base(item.FilePath))
	}

	fmt.Println()
	fmt.Println("📚 Architectural Decision Records")
	fmt.Println("===================================")
	fmt.Println()

	if len(exampleFiles) > 0 {
		fmt.Println("📖 Examples (reference patterns):")
		for _, file := range exampleFiles {
			displayName := strings.TrimPrefix(file, "adr_example-")
			displayName = strings.TrimSuffix(displayName, ".md")
			fmt.Printf("  - %s (%s)\n", displayName, file)
		}
		fmt.Println()
	}

	if len(adrFiles) > 0 {
		fmt.Printf("📋 Project ADRs (%d):\n", len(adrFiles))
		for _, file := range adrFiles {
			displayName := strings.TrimPrefix(file, "adr--")
			if displayName == file {
				displayName = strings.TrimPrefix(file, "adr-")
			}
			displayName = strings.TrimSuffix(displayName, ".md")
			fmt.Printf("  - %s\n", displayName)
		}
	} else {
		fmt.Println("📋 No project ADRs yet.")
		fmt.Println()
		fmt.Println("Agent tip: Create ADRs with 'gears adr new <system-name>'")
		fmt.Println("Document working code patterns to reuse across projects.")
	}

	fmt.Println()

	return nil
}
