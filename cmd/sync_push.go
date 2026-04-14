package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Syluxso/gears/internal/config"
	"github.com/spf13/cobra"
)

var syncPushCmd = &cobra.Command{
	Use:   "push",
	Short: "Push local .gears files to the cloud",
	Long: `Upload all .md and .json files from the .gears directory to the cloud.

Files are organized by workspace ID and tagged for easy filtering.
The sync is incremental - only changed files are updated (based on checksum).`,
	RunE: runSyncPush,
}

func init() {
	syncCmd.AddCommand(syncPushCmd)
}

type pushRequest struct {
	WorkspaceID  string            `json:"workspace_id"`
	FileType     string            `json:"file_type"`
	Filename     string            `json:"filename"`
	RelativePath string            `json:"relative_path"`
	Content      string            `json:"content"`
	Tags         []string          `json:"tags,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

type pushResponse struct {
	UUID     string `json:"uuid"`
	Version  int    `json:"version"`
	Checksum string `json:"checksum"`
}

func runSyncPush(cmd *cobra.Command, args []string) error {
	// Load config
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	// Check authentication
	if err := cfg.RequireAuth(); err != nil {
		return err
	}

	fmt.Println("Scanning .gears directory for files...")

	// Collect files to push
	filesToPush := []string{}
	err = filepath.Walk(".gears", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Only .md and .json files
		ext := filepath.Ext(path)
		if ext != ".md" && ext != ".json" {
			return nil
		}

		// Skip .gearbox directory (config and system files)
		if strings.HasPrefix(path, ".gears"+string(filepath.Separator)+".gearbox") {
			return nil
		}

		filesToPush = append(filesToPush, path)
		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to scan directory: %w", err)
	}

	if len(filesToPush) == 0 {
		fmt.Println("No files to push.")
		return nil
	}

	fmt.Printf("Found %d files to push\n\n", len(filesToPush))

	// Track results
	successCount := 0
	failCount := 0
	typeCounts := make(map[string]int)

	// Push each file
	for i, path := range filesToPush {
		fmt.Printf("[%d/%d] Pushing %s... ", i+1, len(filesToPush), path)

		if err := pushFile(cfg, path, typeCounts); err != nil {
			fmt.Printf("✗ %v\n", err)
			failCount++
		} else {
			fmt.Println("✓")
			successCount++
		}
	}

	// Update last sync time
	cfg.LastSync = time.Now()
	cfg.Save()

	// Print summary
	fmt.Println()
	fmt.Printf("✓ Pushed %d files successfully", successCount)
	if len(typeCounts) > 0 {
		fmt.Print(" (")
		first := true
		for fileType, count := range typeCounts {
			if !first {
				fmt.Print(", ")
			}
			fmt.Printf("%d %s", count, fileType)
			first = false
		}
		fmt.Print(")")
	}
	fmt.Println()

	if failCount > 0 {
		fmt.Printf("✗ %d files failed\n", failCount)
	}

	return nil
}

func pushFile(cfg *config.Config, path string, typeCounts map[string]int) error {
	// Read file content
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	// Get relative path from .gears/
	relPath := strings.TrimPrefix(path, ".gears"+string(filepath.Separator))
	relPath = filepath.ToSlash(relPath) // Convert to forward slashes

	// Detect file type
	fileType := detectFileType(relPath)
	typeCounts[fileType]++

	// Prepare request
	req := pushRequest{
		WorkspaceID:  cfg.WorkspaceID,
		FileType:     fileType,
		Filename:     filepath.Base(path),
		RelativePath: relPath,
		Content:      string(content),
		Tags:         []string{fileType},
		Metadata: map[string]string{
			"cli_version": Version,
			"pushed_at":   time.Now().Format(time.RFC3339),
		},
	}

	// Send to API
	return sendPushRequest(cfg, req)
}

func detectFileType(relPath string) string {
	// Normalize path separators
	relPath = filepath.ToSlash(relPath)

	switch {
	case strings.HasPrefix(relPath, "decisions/"):
		return "adr"
	case strings.HasPrefix(relPath, "story/"):
		return "story"
	case strings.HasPrefix(relPath, "sessions/"):
		return "session"
	case strings.HasPrefix(relPath, "instructions/"):
		return "instruction"
	case strings.HasPrefix(relPath, "memory/"):
		return "memory"
	case strings.HasPrefix(relPath, "artifacts/"):
		return "artifact"
	default:
		return "other"
	}
}

func sendPushRequest(cfg *config.Config, req pushRequest) error {
	// Marshal request
	jsonData, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to encode request: %w", err)
	}

	// Create HTTP request
	apiURL := cfg.APIBaseURL + "/files/push"
	httpReq, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Authorization", "Bearer "+cfg.APIToken)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")

	// Send request
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer resp.Body.Close()

	// Check response
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API error (%d): %s", resp.StatusCode, string(body))
	}

	return nil
}
