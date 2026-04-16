package cmd

import (
	"fmt"
	"strconv"

	"github.com/Syluxso/gears/internal/workspaceregistry"
	"github.com/spf13/cobra"
)

var workspaceCmd = &cobra.Command{
	Use:   "workspace",
	Short: "Manage desktop workspace registry",
	Long: `Manage app-level workspace mappings for desktop usage.

This registry is separate from per-workspace .gears metadata and is used to:
- remember opened workspace paths
- track the active workspace in desktop flows
- support quick workspace switching`,
}

var workspaceOpenCmd = &cobra.Command{
	Use:   "open [path]",
	Short: "Open and register a workspace path",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		targetPath := "."
		if len(args) == 1 {
			targetPath = args[0]
		}

		registry, err := workspaceregistry.Open()
		if err != nil {
			return err
		}
		defer registry.Close()

		entry, err := registry.OpenWorkspace(targetPath)
		if err != nil {
			return err
		}

		fmt.Printf("✓ Opened workspace: %s\n", entry.Name)
		fmt.Printf("  ID:   %d\n", entry.ID)
		fmt.Printf("  Path: %s\n", entry.Path)
		return nil
	},
}

var workspaceListCmd = &cobra.Command{
	Use:   "list",
	Short: "List registered workspaces",
	RunE: func(cmd *cobra.Command, args []string) error {
		registry, err := workspaceregistry.Open()
		if err != nil {
			return err
		}
		defer registry.Close()

		entries, err := registry.List()
		if err != nil {
			return err
		}

		if len(entries) == 0 {
			fmt.Println("No registered workspaces.")
			fmt.Println("Use 'gears workspace open <path>' to add one.")
			return nil
		}

		fmt.Println("📦 Registered Workspaces")
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		for _, e := range entries {
			active := ""
			if e.IsActive {
				active = " [active]"
			}
			fmt.Printf("%d. %s%s\n", e.ID, e.Name, active)
			fmt.Printf("   Path: %s\n", e.Path)
		}

		return nil
	},
}

var workspaceUseCmd = &cobra.Command{
	Use:   "use <workspace-id>",
	Short: "Set active workspace by ID",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := strconv.ParseInt(args[0], 10, 64)
		if err != nil || id <= 0 {
			return fmt.Errorf("invalid workspace id: %s", args[0])
		}

		registry, err := workspaceregistry.Open()
		if err != nil {
			return err
		}
		defer registry.Close()

		entry, err := registry.SetActive(id)
		if err != nil {
			return err
		}

		fmt.Printf("✓ Active workspace: %s\n", entry.Name)
		fmt.Printf("  Path: %s\n", entry.Path)
		return nil
	},
}

var workspaceCurrentCmd = &cobra.Command{
	Use:   "current",
	Short: "Show active workspace",
	RunE: func(cmd *cobra.Command, args []string) error {
		registry, err := workspaceregistry.Open()
		if err != nil {
			return err
		}
		defer registry.Close()

		entry, err := registry.Current()
		if err != nil {
			return err
		}
		if entry == nil {
			fmt.Println("No active workspace.")
			fmt.Println("Use 'gears workspace open <path>' or 'gears workspace use <id>'.")
			return nil
		}

		fmt.Printf("✓ Active workspace: %s\n", entry.Name)
		fmt.Printf("  ID:   %d\n", entry.ID)
		fmt.Printf("  Path: %s\n", entry.Path)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(workspaceCmd)
	workspaceCmd.AddCommand(workspaceOpenCmd)
	workspaceCmd.AddCommand(workspaceListCmd)
	workspaceCmd.AddCommand(workspaceUseCmd)
	workspaceCmd.AddCommand(workspaceCurrentCmd)
}
