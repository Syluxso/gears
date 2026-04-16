package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Syluxso/gears/internal/content"
	"github.com/Syluxso/gears/internal/db"
	"github.com/spf13/cobra"
)

var sessionsCmd = &cobra.Command{
	Use:   "sessions",
	Short: "List and inspect session documentation",
	Long:  "Commands for session docs metadata stored in content_items.",
}

var sessionsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List session docs from content DB",
	RunE:  runSessionsList,
}

func init() {
	rootCmd.AddCommand(sessionsCmd)
	sessionsCmd.AddCommand(sessionsListCmd)
}

func runSessionsList(cmd *cobra.Command, args []string) error {
	if _, err := os.Stat(".gears"); os.IsNotExist(err) {
		return fmt.Errorf(".gears directory not found. Run 'gears init' first")
	}

	if err := db.Initialize(); err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}

	if err := content.SyncFromFiles(db.GetDB()); err != nil {
		return fmt.Errorf("failed to sync content metadata: %w", err)
	}

	items, err := content.GetByType(db.GetDB(), content.TypeSession)
	if err != nil {
		return fmt.Errorf("failed to load sessions: %w", err)
	}

	filtered := make([]content.Item, 0, len(items))
	for _, item := range items {
		name := filepath.Base(item.FilePath)
		base := strings.TrimSuffix(name, filepath.Ext(name))
		if _, err := time.Parse("2006-01-02", base); err == nil {
			filtered = append(filtered, item)
		}
	}
	items = filtered

	if len(items) == 0 {
		fmt.Println("No session docs found in content DB.")
		fmt.Println("Create one with: gears session")
		return nil
	}

	fmt.Println("🗂 Sessions")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	for i, item := range items {
		fmt.Printf("%d. %s\n", i+1, item.Label)
		fmt.Printf("   Status: %s\n", item.State)
		fmt.Printf("   Read: %s\n", item.FilePath)
	}

	fmt.Printf("\nTotal: %d session doc(s)\n", len(items))
	return nil
}
