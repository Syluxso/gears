package cmd

import (
	"fmt"

	"github.com/Syluxso/gears/internal/config"
	"github.com/spf13/cobra"
)

var agentCmd = &cobra.Command{
	Use:   "agent",
	Short: "Manage the default agent identity for this workspace",
	Long: `Set a persistent agent name so you don't have to pass --agent on every backlog command.

Example:
  gears agent set "Grok-Execution"
  gears agent whoami
`,
}

var agentSetCmd = &cobra.Command{
	Use:   "set <name>",
	Short: "Set the default agent name for this .gears workspace",
	Args:  cobra.ExactArgs(1),
	RunE:  runAgentSet,
}

var agentWhoCmd = &cobra.Command{
	Use:   "whoami",
	Short: "Show the current default agent name",
	RunE:  runAgentWhoami,
}

func init() {
	rootCmd.AddCommand(agentCmd)
	agentCmd.AddCommand(agentSetCmd)
	agentCmd.AddCommand(agentWhoCmd)
}

func runAgentSet(cmd *cobra.Command, args []string) error {
	name := args[0]
	if err := config.SetDefaultAgentName(name); err != nil {
		return err
	}
	fmt.Printf("✓ Default agent name set to: %s\n", name)
	fmt.Println("This will be used by default for `gears backlog claim`, `current`, etc.")
	fmt.Println("Override anytime with --agent \"OtherName\"")
	return nil
}

func runAgentWhoami(cmd *cobra.Command, args []string) error {
	name := config.GetDefaultAgentName()
	if name == "" {
		fmt.Println("No default agent name configured for this workspace.")
		fmt.Println("Set one with: gears agent set \"YourAgentName\"")
		return nil
	}
	fmt.Printf("Current default agent: %s\n", name)
	fmt.Println("(Use --agent to override for a single command)")
	return nil
}
