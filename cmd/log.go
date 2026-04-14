package cmd

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/Syluxso/gears/internal/db"
	"github.com/spf13/cobra"
)

var logCmd = &cobra.Command{
	Use:   "log",
	Short: "View command history and activity logs",
	Long:  `Display recent gears command executions and activity.`,
}

var logShowCmd = &cobra.Command{
	Use:   "show [limit]",
	Short: "Show recent command history",
	Long:  `Display the most recent gears commands with details.`,
	Args:  cobra.MaximumNArgs(1),
	RunE:  runLogShow,
}

var logShowVerbose bool

func init() {
	rootCmd.AddCommand(logCmd)
	logCmd.AddCommand(logShowCmd)

	logShowCmd.Flags().BoolVarP(&logShowVerbose, "verbose", "v", false, "Show detailed output including args and errors")
}

func runLogShow(cmd *cobra.Command, args []string) error {
	// Default limit
	limit := 10
	if len(args) > 0 {
		fmt.Sscanf(args[0], "%d", &limit)
	}

	// Database is already initialized by PersistentPreRun
	// Get recent commands
	logs, err := db.GetRecentCommands(limit)
	if err != nil {
		return err
	}

	if len(logs) == 0 {
		fmt.Println("No commands logged yet.")
		return nil
	}

	fmt.Printf("\n📊 Recent Commands (last %d)\n", limit)
	fmt.Println("═══════════════════════════════════════════════════════════")
	fmt.Println()

	for i, log := range logs {
		// Format timestamp
		timeStr := log.Timestamp.Format("2006-01-02 15:04:05")

		// Status indicator
		status := "✓"
		if log.ExitCode != 0 {
			status = "✗"
		}

		// Duration formatting
		durationStr := formatDuration(log.DurationMS)

		// Basic output
		fmt.Printf("%s %s  gears %s  (%s)\n", status, timeStr, log.Command, durationStr)

		// Verbose output
		if logShowVerbose {
			if log.Args != "[]" && log.Args != "" {
				var argsList []string
				if err := json.Unmarshal([]byte(log.Args), &argsList); err == nil && len(argsList) > 0 {
					fmt.Printf("   Args: %v\n", argsList)
				}
			}
			if log.WorkspaceID != "" {
				fmt.Printf("   Workspace: %s\n", log.WorkspaceID)
			}
			if log.CWD != "" {
				fmt.Printf("   Directory: %s\n", log.CWD)
			}
			if log.ErrorMessage != "" {
				fmt.Printf("   Error: %s\n", log.ErrorMessage)
			}
		}

		// Spacing between entries (not for last item)
		if i < len(logs)-1 {
			fmt.Println()
		}
	}

	fmt.Println()
	return nil
}

func formatDuration(ms int64) string {
	if ms < 1000 {
		return fmt.Sprintf("%dms", ms)
	}
	duration := time.Duration(ms) * time.Millisecond
	if duration < time.Minute {
		return fmt.Sprintf("%.1fs", duration.Seconds())
	}
	return fmt.Sprintf("%.1fm", duration.Minutes())
}
