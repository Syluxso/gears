package cmd

import (
	"github.com/spf13/cobra"
)

// Version is set by the main package
var Version = "0.1.0-dev"

var rootCmd = &cobra.Command{
	Use:   "gears",
	Short: "Gears - AI-friendly project documentation and management",
	Long: `Gears is a structured documentation framework that helps AI agents 
and humans maintain shared project understanding across sessions.`,
	Version: Version,
}

// Execute runs the root command
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	// Cobra handles --version automatically, but we can customize the template
	rootCmd.SetVersionTemplate(`{{.Version}}
`)
}
