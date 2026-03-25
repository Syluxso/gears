package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/sylux/gears/internal/config"
)

var (
	syncPullForce bool
)

var syncPullCmd = &cobra.Command{
	Use:   "pull",
	Short: "Pull .gears files from the cloud",
	Long: `Download all files for this workspace from the cloud.

By default, prompts before overwriting local files.
Use --force to overwrite without prompting.`,
	RunE: runSyncPull,
}

func init() {
	syncCmd.AddCommand(syncPullCmd)
	syncPullCmd.Flags().BoolVarP(&syncPullForce, "force", "f", false, "Overwrite local files without prompting")
}

type listResponse struct {
	Data []fileData `json:"data"`
}

type fileData struct {
	UUID         string          `json:"uuid"`
	WorkspaceID  string          `json:"workspace_id"`
	FileType     string          `json:"file_type"`
	Filename     string          `json:"filename"`
	RelativePath string          `json:"relative_path"`
	Content      string          `json:"content"`
	Checksum     string          `json:"checksum"`
	Version      int             `json:"version"`
	SyncedAt     string          `json:"synced_at"`
	Tags         json.RawMessage `json:"tags,omitempty"`
}

func runSyncPull(cmd *cobra.Command, args []string) error {
	// Load config
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	// Check authentication
	if err := cfg.RequireAuth(); err != nil {
		return err
	}

	fmt.Println("Fetching files from cloud...")

	// Fetch file list from API
	files, err := fetchFileList(cfg)
	if err != nil {
		return fmt.Errorf("failed to fetch files: %w", err)
	}

	if len(files) == 0 {
		fmt.Println("No files found in cloud for this workspace.")
		return nil
	}

	fmt.Printf("Found %d files in cloud\n\n", len(files))

	// Track results
	newCount := 0
	updatedCount := 0
	unchangedCount := 0
	skippedCount := 0
	failCount := 0

	// Process each file
	for i, file := range files {
		localPath := filepath.Join(".gears", filepath.FromSlash(file.RelativePath))
		fmt.Printf("[%d/%d] %s... ", i+1, len(files), file.RelativePath)

		// Check if local file exists
		if localData, err := os.ReadFile(localPath); err == nil {
			// File exists - compare checksums
			localChecksum := generateChecksum(localData)

			if localChecksum == file.Checksum {
				// File unchanged, skip silently
				fmt.Println("unchanged")
				unchangedCount++
				continue
			}

			// File modified - check if we should overwrite
			if !syncPullForce {
				fmt.Print("modified, overwrite? [y/N]: ")
				reader := bufio.NewReader(os.Stdin)
				response, _ := reader.ReadString('\n')
				response = strings.ToLower(strings.TrimSpace(response))

				if response != "y" && response != "yes" {
					fmt.Println("skipped")
					skippedCount++
					continue
				}
			}
			updatedCount++
		} else {
			newCount++
		}

		// Write file
		if err := writeFile(localPath, file.Content); err != nil {
			fmt.Printf("✗ %v\n", err)
			failCount++
		} else {
			fmt.Println("✓")
		}
	}

	// Update last sync time
	cfg.LastSync = time.Now()
	cfg.Save()

	// Print summary
	fmt.Println()
	if newCount > 0 {
		fmt.Printf("✓ Downloaded %d new files\n", newCount)
	}
	if updatedCount > 0 {
		fmt.Printf("✓ Updated %d existing files\n", updatedCount)
	}
	if unchangedCount > 0 {
		fmt.Printf("⊘ Skipped %d unchanged files\n", unchangedCount)
	}
	if skippedCount > 0 {
		fmt.Printf("⊘ Skipped %d modified files (user declined)\n", skippedCount)
	}
	if failCount > 0 {
		fmt.Printf("✗ %d files failed\n", failCount)
	}

	return nil
}

func fetchFileList(cfg *config.Config) ([]fileData, error) {
	// Build request URL with workspace filter
	apiURL := fmt.Sprintf("%s/files/list?workspace_id=%s&limit=100", cfg.APIBaseURL, cfg.WorkspaceID)

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+cfg.APIToken)
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error (%d): %s", resp.StatusCode, string(body))
	}

	var listResp listResponse
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return listResp.Data, nil
}

func writeFile(path, content string) error {
	// Create directory if needed
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Write file
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}