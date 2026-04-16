package cmd

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/Syluxso/gears/internal/agent"
	"github.com/Syluxso/gears/internal/db"
	"github.com/Syluxso/gears/internal/events"
	"github.com/Syluxso/gears/internal/monitor"
	"github.com/spf13/cobra"
)

var watchCmd = &cobra.Command{
	Use:   "watch",
	Short: "Manage workspace monitoring",
	Long: `Monitor your workspace for changes and automatically track project activity.

The watch command runs in the background and monitors your workspace for:
- File changes in projects (using git status)
- New or removed project directories
- Git repository activity (commits, branches)

Use 'gears watch start' to begin monitoring, 'gears watch stop' to end it,
and 'gears watch status' to check if monitoring is active.`,
}

var watchStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start workspace monitoring",
	Long: `Start monitoring the workspace (blocking).

This will:
- Scan for existing projects and update the database
- Monitor file changes every 30 seconds
- Check for new/removed projects every 5 minutes
- Fetch git updates every 8 minutes

The watch process will run in your terminal. Press Ctrl+C to stop.

Similar to: php artisan serve, ionic serve, npm run dev`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if isWatchRunning() {
			fmt.Println("❌ Watch is already running")
			startTime := getWatchStartTime()
			if !startTime.IsZero() {
				fmt.Printf("   Started: %s\n", startTime.Format("2006-01-02 15:04:05"))
			}
			fmt.Println("\n   Tip: Run 'gears watch stop' in another terminal to stop it")
			return nil
		}

		fmt.Println("⏳ Starting workspace watch...")

		// Initialize database
		if err := db.Initialize(); err != nil {
			return fmt.Errorf("failed to initialize database: %w", err)
		}

		createdInstructions, err := agent.EnsureCopilotInstructions()
		if err != nil {
			fmt.Printf("Warning: failed to ensure .github/copilot-instructions.md: %v\n", err)
		} else if createdInstructions {
			fmt.Println("✓ Created .github/copilot-instructions.md with Agent Hydration directive")
		}

		// Scan and populate projects
		fmt.Println("📁 Scanning projects directory...")
		if err := db.ScanAndPopulateProjects(); err != nil {
			return fmt.Errorf("failed to scan projects: %w", err)
		}

		// Get project count
		projects, _ := db.GetActiveProjects()
		fmt.Printf("   Found %d project(s)\n", len(projects))

		// Write status file
		if err := writeStatusFile(); err != nil {
			return fmt.Errorf("failed to write status file: %w", err)
		}

		// Log watch_start event
		watchData := events.WatchData{
			SyncEnabled: false,
			Intervals: map[string]string{
				"file_check":   "30s",
				"project_scan": "5m",
				"git_fetch":    "8m",
			},
		}
		_ = events.LogEvent(db.GetDB(), events.EventWatchStart, events.GetWorkspaceUUID(), "", watchData)

		fmt.Println("✓ Watch started")

		// Create monitor with default config
		config := monitor.DefaultConfig()
		mon := monitor.New(config)

		// Setup signal handling for graceful shutdown
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

		// Run monitor in goroutine
		errChan := make(chan error, 1)
		go func() {
			errChan <- mon.Start()
		}()

		// Wait for either signal or error
		select {
		case <-sigChan:
			// User pressed Ctrl+C
			mon.Stop()
			// Wait for monitor to finish
			<-errChan
		case err := <-errChan:
			if err != nil {
				return err
			}
		}

		// Clean up status file
		statusFile := getWatchStatusFile()

		// Log watch_stop event
		stopData := events.WatchData{
			Reason: "user_requested",
		}
		_ = events.LogEvent(db.GetDB(), events.EventWatchStop, events.GetWorkspaceUUID(), "", stopData)

		os.Remove(statusFile)

		fmt.Println("✓ Watch stopped")
		return nil
	},
}

var watchStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop workspace monitoring",
	Long:  `Stop the workspace monitoring daemon gracefully.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if !isWatchRunning() {
			fmt.Println("ℹ️  Watch is not enabled")
			return nil
		}

		// TODO: Send SIGTERM to daemon process when we implement daemon mode
		// For now, just remove status file

		statusFile := getWatchStatusFile()
		if err := os.Remove(statusFile); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to remove status file: %w", err)
		}

		fmt.Println("✓ Watch stopped")
		return nil
	},
}

var watchStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show watch status",
	Long:  `Display the current status of workspace monitoring including uptime, last activity, and monitored projects.`,
	Run: func(cmd *cobra.Command, args []string) {
		if !isWatchRunning() {
			fmt.Println("❌ Watch is not enabled")
			fmt.Println("   Use 'gears watch start' to begin monitoring")
			return
		}

		startTime := getWatchStartTime()

		if startTime.IsZero() {
			fmt.Println("✓ Watch is enabled")
			return
		}

		uptime := time.Since(startTime)

		fmt.Println("✓ Watch is enabled")
		fmt.Printf("  Started: %s\n", startTime.Format("2006-01-02 15:04:05"))
		fmt.Printf("  Uptime: %s\n", formatUptime(uptime))

		// TODO: Query database for monitoring stats
		// - Last file change detected
		// - Last project scan
		// - Last git fetch
		// - Number of active projects
	},
}

func init() {
	rootCmd.AddCommand(watchCmd)
	watchCmd.AddCommand(watchStartCmd)
	watchCmd.AddCommand(watchStopCmd)
	watchCmd.AddCommand(watchStatusCmd)
}

// writeStatusFile creates the watch status file to indicate watch is enabled
func writeStatusFile() error {
	statusFile := getWatchStatusFile()

	// Ensure directory exists
	statusDir := filepath.Dir(statusFile)
	if err := os.MkdirAll(statusDir, 0755); err != nil {
		return err
	}

	// Write timestamp
	now := time.Now().Format(time.RFC3339)
	return os.WriteFile(statusFile, []byte(fmt.Sprintf("started: %s\n", now)), 0644)
}

// getWatchStatusFile returns the path to the watch status file
func getWatchStatusFile() string {
	return filepath.Join(".gears", ".gearbox", "watch.status")
}

// isWatchRunning checks if watch monitoring is enabled
func isWatchRunning() bool {
	statusFile := getWatchStatusFile()
	_, err := os.Stat(statusFile)
	return err == nil
}

// getWatchStartTime returns when watch was started, or zero time if not running
func getWatchStartTime() time.Time {
	statusFile := getWatchStatusFile()

	info, err := os.Stat(statusFile)
	if err != nil {
		return time.Time{}
	}

	return info.ModTime()
}
