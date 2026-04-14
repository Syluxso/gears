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

var storyCmd = &cobra.Command{
	Use:   "story",
	Short: "Manage feature stories and specifications",
	Long:  `Create, list, and manage feature stories in .gears/story/.`,
}

var storyNewCmd = &cobra.Command{
	Use:   "new <name>",
	Short: "Create a new story file with template",
	Long: `Creates a new story file in .gears/story/ using the format story--<name>.md.
	
The story file includes a template with sections for description, acceptance criteria, and technical notes.`,
	Args: cobra.ExactArgs(1),
	RunE: runStoryNew,
}

var storyListCmd = &cobra.Command{
	Use:   "list",
	Short: "Show all stories (active/queued/completed)",
	Long:  `Lists all story files in .gears/story/ grouped by status.`,
	RunE:  runStoryList,
}

func init() {
	rootCmd.AddCommand(storyCmd)
	storyCmd.AddCommand(storyNewCmd)
	storyCmd.AddCommand(storyListCmd)
}

func runStoryNew(cmd *cobra.Command, args []string) error {
	// Check if .gears exists
	if _, err := os.Stat(".gears"); os.IsNotExist(err) {
		return fmt.Errorf(".gears directory not found. Run 'gears init' first")
	}

	// Check if story directory exists
	storyDir := filepath.Join(".gears", "story")
	if _, err := os.Stat(storyDir); os.IsNotExist(err) {
		return fmt.Errorf(".gears/story directory not found")
	}

	// Get story name and create filename
	name := args[0]
	slug := content.NormalizeSlug(name)
	storyFile, err := content.BuildDefaultFilePath(content.TypeStory, slug)
	if err != nil {
		return err
	}

	if err := db.Initialize(); err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}

	item, err := content.CreateItem(db.GetDB(), content.TypeStory, "md", name, slug, "pending", storyFile)
	if err != nil {
		return fmt.Errorf("failed to create story metadata: %w", err)
	}

	// Check if file already exists
	if _, err := os.Stat(storyFile); err == nil {
		return fmt.Errorf("story file already exists: %s", storyFile)
	}

	// Create story file with template
	today := time.Now().Format("2006-01-02")
	storyTemplate := fmt.Sprintf(`# Story: %s

**Status:** Draft
**Project:** _[project name]_
**Created:** %s

## What We're Building

_[Plain English description of the feature. One paragraph.]_

## Why

_[Business or user reason. Why does this matter?]_

## Acceptance Criteria

- [ ] _[Criterion 1]_
- [ ] _[Criterion 2]_
- [ ] _[Criterion 3]_

## Technical Notes

_[Architectural considerations, constraints, approach decisions, related ADRs.]_

## Related

- ADR: _[link if applicable]_
- Artifact: _[link if applicable]_
`, name, today)

	if err := os.WriteFile(storyFile, []byte(storyTemplate), 0644); err != nil {
		_ = content.SyncFromFiles(db.GetDB())
		return fmt.Errorf("failed to create story file: %w", err)
	}

	_ = content.SyncFromFiles(db.GetDB())
	_ = content.UpdateSyncMetadata(db.GetDB(), item.UUID, "")

	fmt.Printf("✓ Created story: %s\n", storyFile)
	fmt.Println("\nAgent next:")
	fmt.Println("  1. Populate from user's request:")
	fmt.Println("     - \"What We're Building\" (feature description)")
	fmt.Println("     - \"Why\" (user need or business value)")
	fmt.Println("     - \"Acceptance Criteria\" (3-5 testable conditions)")
	fmt.Println("     - \"Technical Notes\" (approach and constraints)")
	fmt.Println("  2. Add entry to .gears/story/index.md under \"Queued Stories\"")
	fmt.Println("  3. Set Status to \"Ready\" when complete")

	return nil
}

func runStoryList(cmd *cobra.Command, args []string) error {
	// Check if .gears exists
	if _, err := os.Stat(".gears"); os.IsNotExist(err) {
		return fmt.Errorf(".gears directory not found. Run 'gears init' first")
	}

	// Check if story directory exists
	storyDir := filepath.Join(".gears", "story")
	if _, err := os.Stat(storyDir); os.IsNotExist(err) {
		return fmt.Errorf(".gears/story directory not found")
	}

	if err := db.Initialize(); err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}

	if err := content.SyncFromFiles(db.GetDB()); err != nil {
		return fmt.Errorf("failed to sync stories: %w", err)
	}

	// Group stories by status
	type Story struct {
		Name   string
		Status string
		File   string
	}

	var stories []Story

	items, err := content.GetByType(db.GetDB(), content.TypeStory)
	if err != nil {
		return fmt.Errorf("failed to query stories: %w", err)
	}

	for _, item := range items {
		stories = append(stories, Story{
			Name:   item.Label,
			Status: item.State,
			File:   filepath.Base(item.FilePath),
		})
	}

	if len(stories) == 0 {
		fmt.Println("No stories found in .gears/story/")
		fmt.Println("\nCreate your first story with: gears story new <name>")
		return nil
	}

	// Group and display by status
	activeStories := []Story{}
	queuedStories := []Story{}
	inProgressStories := []Story{}
	completedStories := []Story{}
	otherStories := []Story{}

	for _, story := range stories {
		switch strings.ToLower(story.Status) {
		case "in progress", "in_progress", "current":
			inProgressStories = append(inProgressStories, story)
		case "ready":
			activeStories = append(activeStories, story)
		case "queued", "draft", "pending", "planning", "created":
			queuedStories = append(queuedStories, story)
		case "done", "completed":
			completedStories = append(completedStories, story)
		default:
			otherStories = append(otherStories, story)
		}
	}

	// Display results
	fmt.Println("\n📖 Stories")
	fmt.Println("==========")

	if len(inProgressStories) > 0 {
		fmt.Println("\n🔄 In Progress:")
		for _, s := range inProgressStories {
			fmt.Printf("  - %s (%s)\n", s.Name, s.File)
		}
	}

	if len(activeStories) > 0 {
		fmt.Println("\n✅ Ready:")
		for _, s := range activeStories {
			fmt.Printf("  - %s (%s)\n", s.Name, s.File)
		}
	}

	if len(queuedStories) > 0 {
		fmt.Println("\n📋 Queued/Draft:")
		for _, s := range queuedStories {
			fmt.Printf("  - %s (%s)\n", s.Name, s.File)
		}
	}

	if len(completedStories) > 0 {
		fmt.Println("\n✔️  Completed:")
		for _, s := range completedStories {
			fmt.Printf("  - %s (%s)\n", s.Name, s.File)
		}
	}

	if len(otherStories) > 0 {
		fmt.Println("\n❓ Other:")
		for _, s := range otherStories {
			fmt.Printf("  - %s [%s] (%s)\n", s.Name, s.Status, s.File)
		}
	}

	fmt.Printf("\nTotal: %d stories\n", len(stories))

	// Context-aware agent tips
	if len(inProgressStories) > 0 {
		fmt.Printf("\n⚠️  Active: %s\n", inProgressStories[0].Name)
		fmt.Println("Agent tip: Continue working on this story's acceptance criteria.")
	} else if len(activeStories) > 0 || len(queuedStories) > 0 {
		fmt.Println("\nAgent tip: Start a story by updating its Status to \"In Progress\" in the file.")
	}

	return nil
}
