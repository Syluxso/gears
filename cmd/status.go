package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Syluxso/gears/internal/content"
	"github.com/Syluxso/gears/internal/db"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show workspace status",
	Long: `Display an overview of your workspace including:
- Workspace configuration
- Active projects and their last activity
- Watch monitoring status`,
	Run: func(cmd *cobra.Command, args []string) {
		// Get current directory
		cwd, err := os.Getwd()
		if err != nil {
			fmt.Println("❌ Error getting current directory:", err)
			return
		}

		// Get workspace name from directory
		workspaceName := filepath.Base(cwd)

		// Display workspace info
		fmt.Println("📦 Workspace Status")
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		fmt.Printf("Name:     %s\n", workspaceName)
		fmt.Printf("Path:     %s\n", cwd)
		fmt.Println()

		// Display watch status
		fmt.Println("⚙️  Watch Status")
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		if isWatchRunning() {
			startTime := getWatchStartTime()

			if startTime.IsZero() {
				fmt.Println("Status:   ✓ Enabled")
			} else {
				uptime := time.Since(startTime)
				fmt.Println("Status:   ✓ Enabled")
				fmt.Printf("Started:  %s\n", startTime.Format("2006-01-02 15:04:05"))
				fmt.Printf("Uptime:   %s\n", formatUptime(uptime))
			}
		} else {
			fmt.Println("Status:   ○ Not enabled")
			fmt.Println("          Use 'gears watch start' to begin monitoring")
		}
		fmt.Println()

		// Initialize database and get projects
		db.Initialize()
		projects, err := db.GetActiveProjects()
		if err != nil {
			fmt.Println("❌ Error loading projects:", err)
			return
		}

		// Display projects
		fmt.Println("📁 Active Projects")
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		if len(projects) == 0 {
			fmt.Println("No active projects found")
			fmt.Println()
			fmt.Println("Projects are auto-discovered from the projects/ directory.")
			fmt.Println("Run 'gears watch start' to begin monitoring.")
		} else {
			for i, project := range projects {
				// Format project type
				projectType := project.Language
				if project.Framework != "" {
					projectType = fmt.Sprintf("%s (%s)", project.Language, project.Framework)
				}

				// Display project info
				fmt.Printf("%d. %s\n", i+1, project.Name)
				fmt.Printf("   Type:     %s\n", projectType)

				// Show git branch if available
				if project.GitCurrentBranch != "" {
					fmt.Printf("   Branch:   %s", project.GitCurrentBranch)
					if project.GitLastCommitHash != "" {
						// Show short commit hash (first 7 chars)
						commitShort := project.GitLastCommitHash
						if len(commitShort) > 7 {
							commitShort = commitShort[:7]
						}
						fmt.Printf(" @ %s", commitShort)
					}
					fmt.Println()
				}

				// Show last activity if available
				if project.LastActivityAt != nil && !project.LastActivityAt.IsZero() {
					timeSince := time.Since(*project.LastActivityAt)
					fmt.Printf("   Activity: %s", formatTimeAgo(timeSince))
					fmt.Println()
				}

				fmt.Println()
			}

			fmt.Printf("Total: %d active project(s)\n", len(projects))
		}
		fmt.Println()

		// Content snapshot (stories, ADRs, consults) — fulfills consult story AC
		showContentSnapshot()

		// Current claimed backlog item (the thing an agent should be focused on)
		showCurrentBacklogWork()
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
}

// formatTimeAgo formats a duration as "X ago" (e.g., "2 minutes ago", "3 days ago")
func formatTimeAgo(d time.Duration) string {
	if d < time.Minute {
		return "just now"
	}
	if d < time.Hour {
		mins := int(d.Minutes())
		if mins == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", mins)
	}
	if d < 24*time.Hour {
		hours := int(d.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	}
	days := int(d.Hours() / 24)
	if days == 1 {
		return "1 day ago"
	}
	if days < 30 {
		return fmt.Sprintf("%d days ago", days)
	}
	months := days / 30
	if months == 1 {
		return "1 month ago"
	}
	if months < 12 {
		return fmt.Sprintf("%d months ago", months)
	}
	years := months / 12
	if years == 1 {
		return "1 year ago"
	}
	return fmt.Sprintf("%d years ago", years)
}

// formatUptime formats a duration in a human-readable way
func formatUptime(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%d seconds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%d minutes", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		hours := int(d.Hours())
		minutes := int(d.Minutes()) % 60
		return fmt.Sprintf("%dh %dm", hours, minutes)
	}
	days := int(d.Hours() / 24)
	hours := int(d.Hours()) % 24
	return fmt.Sprintf("%dd %dh", days, hours)
}

// showContentSnapshot prints a brief overview of DB-tracked stories, ADRs and consults.
// Added to satisfy the "Consults appear in `gears status` ..." acceptance criterion.
func showContentSnapshot() {
	if db.GetDB() == nil {
		// DB not initialized (e.g. no .gears yet); skip silently for status overview
		return
	}

	if err := content.SyncFromFiles(db.GetDB()); err != nil {
		// Non-fatal for status
		return
	}

	fmt.Println("📖 Content Snapshot")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	types := []struct {
		typ   string
		label string
	}{
		{content.TypeStory, "Stories"},
		{content.TypeADR, "ADRs"},
		{content.TypeConsult, "Consults"},
		{content.TypeBacklog, "Backlog Items"},
	}

	for _, t := range types {
		items, err := content.GetByType(db.GetDB(), t.typ)
		if err != nil || len(items) == 0 {
			fmt.Printf("%s: 0\n", t.label)
			continue
		}

		// GetByType is ASC by created; show the most recent few
		recent := items
		if len(recent) > 3 {
			recent = recent[len(recent)-3:]
		}

		fmt.Printf("%s: %d total\n", t.label, len(items))
		for _, it := range recent {
			age := ""
			if !it.UpdatedAt.IsZero() {
				age = " (" + formatTimeAgo(time.Since(it.UpdatedAt)) + ")"
			}
			state := it.State
			if state != "" && state != "created" && state != "logged" && state != "open" {
				state = " [" + state + "]"
			} else {
				state = ""
			}
			fmt.Printf("   • %s%s%s\n", it.Label, state, age)
		}
		if len(items) > 3 {
			fmt.Printf("   … and %d more\n", len(items)-3)
		}
	}

	fmt.Println()
	fmt.Println("Tip: Use `gears story list`, `gears consult latest` (try --full/--follow/--weaponsfree during active threads) for details.")
	fmt.Println()
}

// showCurrentBacklogWork shows the single most important thing: what (if anything) is currently claimed.
func showCurrentBacklogWork() {
	if db.GetDB() == nil {
		return
	}

	// We don't have a forced agent name in status; show a summary instead of assuming one agent.
	items, err := db.ListBacklogItems("")
	if err != nil || len(items) == 0 {
		return
	}

	// Find any claimed/in_progress
	var current *db.BacklogItem
	pendingCount := 0
	for i := range items {
		it := &items[i]
		if it.Status == "claimed" || it.Status == "in_progress" {
			if current == nil {
				current = it
			}
		}
		if it.Status == "pending" {
			pendingCount++
		}
	}

	fmt.Println("📌 Backlog Work")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	if current != nil {
		claimStr := ""
		if current.ClaimedBy != "" {
			claimStr = " (claimed by " + current.ClaimedBy + ")"
		}
		fmt.Printf("Current: [%d] %s%s\n", current.ID, current.Label, claimStr)
	} else {
		fmt.Println("Current: none claimed")
	}

	fmt.Printf("Pending in backlog: %d\n", pendingCount)
	fmt.Println("Use: gears backlog current   or   gears backlog list")
	fmt.Println()
}
