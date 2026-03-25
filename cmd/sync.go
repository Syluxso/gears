package cmd

import (
	"github.com/spf13/cobra"
)

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Synchronize .gears files with the cloud",
	Long: `Push or pull .gears project files to/from the web platform.

Use 'gears sync push' to upload files
Use 'gears sync pull' to download files

Requires authentication (run 'gears auth' first).`,
}

func init() {
	rootCmd.AddCommand(syncCmd)
}
