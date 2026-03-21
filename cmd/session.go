package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
)

var sessionCmd = &cobra.Command{
	Use:   "session",
	Short: "Create today's session file if it doesn't exist",
	Long: `Creates a session file for today in .gears/sessions/ using the format YYYY-MM-DD.md.
	
If the file already exists, displays a message and does nothing.
Session files document what was worked on during a session.`,
	RunE: runSession,
}

func init() {
	rootCmd.AddCommand(sessionCmd)
}

func runSession(cmd *cobra.Command, args []string) error {
	// Check if .gears exists
	if _, err := os.Stat(".gears"); os.IsNotExist(err) {
		return fmt.Errorf(".gears directory not found. Run 'gears init' first")
	}

	// Check if sessions directory exists
	sessionsDir := filepath.Join(".gears", "sessions")
	if _, err := os.Stat(sessionsDir); os.IsNotExist(err) {
		return fmt.Errorf(".gears/sessions directory not found")
	}

	// Get today's date
	today := time.Now().Format("2006-01-02")
	sessionFile := filepath.Join(sessionsDir, today+".md")

	// Check if file already exists
	if _, err := os.Stat(sessionFile); err == nil {
		fmt.Printf("✓ Session file already exists: %s\n", sessionFile)
		fmt.Println("\nAgent: Update this file NOW with current session state:")
		fmt.Println("  1. Read .gears/context/index.md for project/phase info")
		fmt.Println("  2. Review conversation: what tasks were completed?")
		fmt.Println("  3. Document any decisions made or problems solved")
		fmt.Println("  4. List files created/modified in this session")
		fmt.Println("  5. Note what should be picked up next")
		fmt.Println("\nRead the file first, then append/update appropriate sections.")
		return nil
	}

	// Create session file with template
	content := fmt.Sprintf(`# Session: %s

**Project:** _[project name]_
**Phase:** _[current phase]_
**Agent/Human:** _[who did the work]_

## What Was Done

- _[List tasks completed]_

## Decisions Made

- _[Any significant choices (write an ADR if architectural)]_

## Problems Encountered

- _[Issues hit and how they were resolved]_

## Code Changes

- _[Files created/modified (brief summary)]_

## Next Session Should

- Pick up from: _[specific task]_
- Watch out for: _[any gotchas]_
`, today)

	if err := os.WriteFile(sessionFile, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to create session file: %w", err)
	}

	fmt.Printf("✓ Created session file: %s\n", sessionFile)
	fmt.Println("\nAgent: Update this file NOW with current session state:")
	fmt.Println("  1. Read .gears/context/index.md for project/phase info")
	fmt.Println("  2. Review conversation: what tasks were completed?")
	fmt.Println("  3. Document any decisions made or problems solved")
	fmt.Println("  4. List files created/modified in this session")
	fmt.Println("  5. Note what should be picked up next")
	fmt.Println("\nRead the file first, then populate the template sections.")

	return nil
}
