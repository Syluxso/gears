package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/Syluxso/gears/internal/db"
	"github.com/Syluxso/gears/internal/events"
	"github.com/spf13/cobra"
)

var (
	eventsType    string
	eventsProject string
	eventsSince   string
	eventsUntil   string
)

var eventsCmd = &cobra.Command{
	Use:   "events",
	Short: "View and manage workspace events",
	Long: `View workspace activity events including:
- Commands executed
- File changes detected  
- Git commits and fetches
- Projects added/removed
- Watch start/stop

Use 'gears events show' to view recent events,
'gears events stats' for statistics,
and 'gears events export' to export as JSONL.`,
}

var eventsShowCmd = &cobra.Command{
	Use:   "show [count]",
	Short: "Show recent events",
	Long: `Display recent workspace events with optional filters.

Examples:
  gears events show              # Last 50 events
  gears events show 100          # Last 100 events
  gears events show --type command
  gears events show --project gears
  gears events show --since 1h
  gears events show --type git_commit --since 1d`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := db.Initialize(); err != nil {
			fmt.Println("❌ Failed to initialize database:", err)
			return
		}

		limit := 50
		if len(args) > 0 {
			fmt.Sscanf(args[0], "%d", &limit)
		}

		// Parse time filters
		var since, until *time.Time
		if eventsSince != "" {
			t, err := parseDuration(eventsSince)
			if err == nil {
				since = &t
			}
		}
		if eventsUntil != "" {
			t, err := parseDuration(eventsUntil)
			if err == nil {
				until = &t
			}
		}

		// Get events
		eventsList, err := events.GetEvents(db.GetDB(), limit, eventsType, eventsProject, since, until)
		if err != nil {
			fmt.Println("❌ Failed to load events:", err)
			return
		}

		if len(eventsList) == 0 {
			fmt.Println("No events found")
			return
		}

		fmt.Printf("📋 Events (showing %d)\n", len(eventsList))
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

		for _, e := range eventsList {
			formatEvent(e)
		}
	},
}

var eventsStatsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show event statistics",
	Long:  `Display statistics about workspace events grouped by type.`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := db.Initialize(); err != nil {
			fmt.Println("❌ Failed to initialize database:", err)
			return
		}

		// Parse time filter
		var since *time.Time
		if eventsSince != "" {
			t, err := parseDuration(eventsSince)
			if err == nil {
				since = &t
			}
		}

		stats, err := events.GetEventStats(db.GetDB(), since)
		if err != nil {
			fmt.Println("❌ Failed to load stats:", err)
			return
		}

		if len(stats) == 0 {
			fmt.Println("No events found")
			return
		}

		fmt.Println("📊 Event Statistics")
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

		total := 0
		for eventType, count := range stats {
			emoji := getEventEmoji(eventType)
			fmt.Printf("%s %-20s %6d\n", emoji, eventType, count)
			total += count
		}

		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		fmt.Printf("Total:                    %6d\n", total)
	},
}

var eventsExportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export events as JSONL",
	Long: `Export workspace events as JSON Lines format (one JSON object per line).

Examples:
  gears events export > events.jsonl
  gears events export --since 1d --type git_commit > commits.jsonl`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := db.Initialize(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to initialize database: %v\n", err)
			os.Exit(1)
		}

		// Parse time filters
		var since, until *time.Time
		if eventsSince != "" {
			t, err := parseDuration(eventsSince)
			if err == nil {
				since = &t
			}
		}
		if eventsUntil != "" {
			t, err := parseDuration(eventsUntil)
			if err == nil {
				until = &t
			}
		}

		// Get events (no limit for export)
		eventsList, err := events.GetEvents(db.GetDB(), 0, eventsType, eventsProject, since, until)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to load events: %v\n", err)
			os.Exit(1)
		}

		// Output JSONL
		for _, e := range eventsList {
			// Parse data JSON
			var dataMap map[string]interface{}
			if e.Data != "" {
				json.Unmarshal([]byte(e.Data), &dataMap)
			}

			// Create output object
			output := map[string]interface{}{
				"timestamp":      e.Timestamp.Format(time.RFC3339),
				"event_type":     e.EventType,
				"workspace_uuid": e.WorkspaceUUID,
				"project_uuid":   e.ProjectUUID,
				"data":           dataMap,
			}

			jsonBytes, _ := json.Marshal(output)
			fmt.Println(string(jsonBytes))
		}
	},
}

func init() {
	rootCmd.AddCommand(eventsCmd)
	eventsCmd.AddCommand(eventsShowCmd)
	eventsCmd.AddCommand(eventsStatsCmd)
	eventsCmd.AddCommand(eventsExportCmd)

	// Flags for show command
	eventsShowCmd.Flags().StringVar(&eventsType, "type", "", "Filter by event type")
	eventsShowCmd.Flags().StringVar(&eventsProject, "project", "", "Filter by project UUID or name")
	eventsShowCmd.Flags().StringVar(&eventsSince, "since", "", "Show events since (e.g., 1h, 2d, 2026-04-14)")
	eventsShowCmd.Flags().StringVar(&eventsUntil, "until", "", "Show events until (e.g., now, 2026-04-14)")

	// Flags for stats command
	eventsStatsCmd.Flags().StringVar(&eventsSince, "since", "", "Stats since (e.g., 1h, 1d, 7d)")

	// Flags for export command
	eventsExportCmd.Flags().StringVar(&eventsType, "type", "", "Filter by event type")
	eventsExportCmd.Flags().StringVar(&eventsProject, "project", "", "Filter by project")
	eventsExportCmd.Flags().StringVar(&eventsSince, "since", "", "Export events since")
	eventsExportCmd.Flags().StringVar(&eventsUntil, "until", "", "Export events until")
}

// formatEvent displays a single event with formatting
func formatEvent(e events.Event) {
	emoji := getEventEmoji(e.EventType)
	timeStr := e.Timestamp.Format("2006-01-02 15:04:05")

	fmt.Printf("%s %s - %s\n", emoji, timeStr, e.EventType)

	// Parse and display relevant data fields
	if e.Data != "" {
		var dataMap map[string]interface{}
		if err := json.Unmarshal([]byte(e.Data), &dataMap); err == nil {
			displayEventData(e.EventType, dataMap)
		}
	}

	fmt.Println()
}

// displayEventData shows relevant fields from event data based on type
func displayEventData(eventType string, data map[string]interface{}) {
	switch eventType {
	case events.EventCommand:
		if cmd, ok := data["command"].(string); ok {
			fmt.Printf("   Command: %s\n", cmd)
		}
		if exitCode, ok := data["exit_code"].(float64); ok {
			status := "✓"
			if exitCode != 0 {
				status = "✗"
			}
			fmt.Printf("   Status: %s (exit code: %.0f)\n", status, exitCode)
		}

	case events.EventFileChange:
		if project, ok := data["project"].(string); ok {
			fmt.Printf("   Project: %s\n", project)
		}
		if count, ok := data["files_changed"].(float64); ok {
			fmt.Printf("   Files changed: %.0f\n", count)
		}

	case events.EventGitCommit:
		if msg, ok := data["message"].(string); ok {
			fmt.Printf("   Message: %s\n", msg)
		}
		if author, ok := data["author"].(string); ok {
			fmt.Printf("   Author: %s\n", author)
		}
		if hash, ok := data["commit_hash"].(string); ok {
			if len(hash) > 7 {
				hash = hash[:7]
			}
			fmt.Printf("   Commit: %s\n", hash)
		}

	case events.EventGitFetch:
		if remote, ok := data["remote"].(string); ok {
			fmt.Printf("   Remote: %s\n", remote)
		}
		if commits, ok := data["new_commits"].(float64); ok {
			fmt.Printf("   New commits: %.0f\n", commits)
		}

	case events.EventProjectAdded, events.EventProjectRemoved:
		if project, ok := data["project"].(string); ok {
			fmt.Printf("   Project: %s\n", project)
		}
		if path, ok := data["path"].(string); ok {
			fmt.Printf("   Path: %s\n", path)
		}
	}
}

// getEventEmoji returns an emoji for each event type
func getEventEmoji(eventType string) string {
	switch eventType {
	case events.EventCommand:
		return "⚡"
	case events.EventFileChange:
		return "📝"
	case events.EventGitCommit:
		return "🔧"
	case events.EventGitFetch:
		return "🌐"
	case events.EventProjectAdded:
		return "➕"
	case events.EventProjectRemoved:
		return "➖"
	case events.EventWatchStart:
		return "▶️ "
	case events.EventWatchStop:
		return "⏹️ "
	default:
		return "•"
	}
}

// parseDuration parses a duration string like "1h", "2d", "30m" or absolute time "2026-04-14"
func parseDuration(s string) (time.Time, error) {
	// Try parsing as absolute time first
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t, nil
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}

	// Parse as duration (1h, 2d, etc.)
	s = strings.TrimSpace(s)
	if s == "now" {
		return time.Now(), nil
	}

	// Extract number and unit
	var value int
	var unit string
	fmt.Sscanf(s, "%d%s", &value, &unit)

	var duration time.Duration
	switch unit {
	case "s", "sec", "second", "seconds":
		duration = time.Duration(value) * time.Second
	case "m", "min", "minute", "minutes":
		duration = time.Duration(value) * time.Minute
	case "h", "hour", "hours":
		duration = time.Duration(value) * time.Hour
	case "d", "day", "days":
		duration = time.Duration(value) * 24 * time.Hour
	case "w", "week", "weeks":
		duration = time.Duration(value) * 7 * 24 * time.Hour
	default:
		return time.Time{}, fmt.Errorf("invalid duration: %s", s)
	}

	return time.Now().Add(-duration), nil
}
