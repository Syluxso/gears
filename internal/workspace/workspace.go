package workspace

import (
	"fmt"
	"os"
	"path/filepath"
)

// FindWorkspaceRoot searches upward from the current directory to find the .gears workspace root.
// Returns the absolute path to the workspace root, or an error if no workspace is found.
func FindWorkspaceRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get current directory: %w", err)
	}

	// Start from current directory, traverse up
	dir := cwd
	for {
		gearsPath := filepath.Join(dir, ".gears")

		// Check if .gears exists and is a directory
		if info, err := os.Stat(gearsPath); err == nil && info.IsDir() {
			return dir, nil // Found workspace root!
		}

		// Go up one directory
		parent := filepath.Dir(dir)

		// Reached filesystem root? (parent == dir means we can't go higher)
		if parent == dir {
			return "", fmt.Errorf("not a gears workspace (or any parent up to mount point)")
		}

		dir = parent
	}
}

// GetWorkspaceName returns the name of the workspace (directory name of workspace root)
func GetWorkspaceName(workspaceRoot string) string {
	return filepath.Base(workspaceRoot)
}

// IsWorkspaceRoot checks if the given directory is a workspace root (contains .gears/)
func IsWorkspaceRoot(dir string) bool {
	gearsPath := filepath.Join(dir, ".gears")
	info, err := os.Stat(gearsPath)
	return err == nil && info.IsDir()
}
