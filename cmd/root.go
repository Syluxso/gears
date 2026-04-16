package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/Syluxso/gears/internal/config"
	"github.com/Syluxso/gears/internal/db"
	"github.com/Syluxso/gears/internal/events"
	"github.com/Syluxso/gears/internal/workspace"
	"github.com/spf13/cobra"
)

// Version is set by the main package
var Version = "0.2.0-dev"

// Command execution context
var (
	cmdStartTime time.Time
	cmdArgs      []string
	cmdFullPath  string
)

var rootCmd = &cobra.Command{
	Use:   "gears",
	Short: "Gears - AI-friendly project documentation and management",
	Long: `Gears is a structured documentation framework that helps AI agents 
and humans maintain shared project understanding across sessions.`,
	Version: Version,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// Record start time for duration calculation
		cmdStartTime = time.Now()
		cmdArgs = args

		// Capture full command path (e.g., "log show" instead of just "log")
		if cmd.Parent() != nil && cmd.Parent().Name() != "gears" {
			cmdFullPath = cmd.Parent().Name() + " " + cmd.Name()
		} else {
			cmdFullPath = cmd.Name()
		}

		// Skip workspace discovery for init and workspace registry commands.
		skipWorkspaceDiscovery := cmd.Name() == "init" ||
			cmd.Name() == "workspace" ||
			(cmd.Parent() != nil && cmd.Parent().Name() == "workspace")

		if !skipWorkspaceDiscovery {
			// Find workspace root by traversing up from CWD
			workspaceRoot, err := workspace.FindWorkspaceRoot()
			if err != nil {
				fmt.Fprintf(os.Stderr, "❌ %v\n", err)
				fmt.Fprintf(os.Stderr, "   Use 'gears init' to create a workspace\n")
				os.Exit(1)
			}

			// Change to workspace root so all relative paths work correctly
			if err := os.Chdir(workspaceRoot); err != nil {
				fmt.Fprintf(os.Stderr, "❌ Failed to change to workspace root: %v\n", err)
				os.Exit(1)
			}
		}

		// Initialize database (silently fails if not in .gears workspace)
		_ = db.Initialize()
	},
	PersistentPostRun: func(cmd *cobra.Command, args []string) {
		// Logging is now handled in Execute() to properly capture exit codes
		// This hook is kept empty for potential future use
	},
}

// Execute runs the root command
func Execute() error {
	err := rootCmd.Execute()

	// Log the command AFTER execution completes (so we can capture exit code)
	if cmdStartTime.IsZero() {
		// PersistentPreRun didn't run (e.g., --help), skip logging
		return err
	}

	duration := time.Since(cmdStartTime).Milliseconds()

	// Get workspace ID
	workspaceID := ""
	if cfg, err := config.Load(); err == nil {
		workspaceID = cfg.WorkspaceID
	}

	// Get current working directory
	cwd, _ := os.Getwd()

	// Use captured command path (set in PersistentPreRun)
	cmdName := cmdFullPath
	if cmdName == "" {
		// Fallback if PersistentPreRun didn't capture it
		if len(os.Args) > 1 {
			cmdName = os.Args[1]
		} else {
			cmdName = rootCmd.Name()
		}
	}

	// Determine exit code and error message
	exitCode := 0
	errMsg := ""
	if err != nil {
		exitCode = 1
		errMsg = err.Error()
	}

	// Log the command
	log := &db.CommandLog{
		Command:      cmdName,
		Args:         db.ArgsToJSON(cmdArgs),
		ExitCode:     exitCode,
		Timestamp:    cmdStartTime,
		DurationMS:   duration,
		CWD:          cwd,
		WorkspaceID:  workspaceID,
		ErrorMessage: errMsg,
	}

	// Log to database (ignore errors to not disrupt command execution)
	_ = db.LogCommand(log)

	// Also emit a command event into the activity stream.
	_ = events.LogEvent(db.GetDB(), events.EventCommand, events.GetWorkspaceUUID(), "", events.CommandData{
		Command:    cmdName,
		Args:       db.ArgsToJSON(cmdArgs),
		ExitCode:   exitCode,
		DurationMS: duration,
		CWD:        cwd,
	})

	// Close database connection
	_ = db.Close()

	return err
}

func init() {
	// Cobra handles --version automatically, but we can customize the template
	rootCmd.SetVersionTemplate(`{{.Version}}
`)
}
