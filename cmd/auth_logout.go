package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/Syluxso/gears/internal/config"
)

var authLogoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Clear authentication token",
	Long: `Remove the stored API token from your configuration.

You will need to run 'gears auth' again to re-authenticate.`,
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := config.Load()
		if err != nil {
			fmt.Println("✗ Error loading config:", err)
			return
		}

		if cfg.APIToken == "" {
			fmt.Println("✓ Not currently authenticated")
			return
		}

		// Clear the token
		cfg.APIToken = ""
		
		if err := cfg.Save(); err != nil {
			fmt.Println("✗ Error saving config:", err)
			return
		}

		fmt.Println("✓ Logged out successfully")
		fmt.Println("\nRun 'gears auth' to authenticate again.")
	},
}

func init() {
	authCmd.AddCommand(authLogoutCmd)
}
