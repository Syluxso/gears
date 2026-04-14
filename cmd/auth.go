package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/Syluxso/gears/internal/config"
	"github.com/spf13/cobra"
)

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Authenticate with the gears-hub API",
	Long: `Authenticate with the gears-hub web platform to enable file sync.

This command will:
1. Open your browser to the token creation page
2. Prompt you to paste your API token
3. Validate the token
4. Store it securely in .gears/.gearbox/config.json

Required token abilities: files:read, files:write`,
	RunE: runAuth,
}

func init() {
	rootCmd.AddCommand(authCmd)
}

type userResponse struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

func runAuth(cmd *cobra.Command, args []string) error {
	// Load existing config
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	// Check if already authenticated
	if cfg.APIToken != "" {
		fmt.Printf("Already authenticated. Refresh token? [y/N]: ")
		reader := bufio.NewReader(os.Stdin)
		response, _ := reader.ReadString('\n')
		response = strings.ToLower(strings.TrimSpace(response))

		if response != "y" && response != "yes" {
			fmt.Println("Authentication cancelled.")
			return nil
		}
	}

	// Open browser to token page
	tokenURL := strings.Replace(cfg.APIBaseURL, "/api/v1", "/tokens", 1)
	fmt.Printf("Opening browser to: %s\n\n", tokenURL)

	if err := openBrowser(tokenURL); err != nil {
		fmt.Printf("⚠ Could not open browser automatically: %v\n", err)
		fmt.Printf("Please open this URL manually: %s\n\n", tokenURL)
	}

	fmt.Println("Create a new API token with the following abilities:")
	fmt.Println("  - files:read")
	fmt.Println("  - files:write")
	fmt.Println()
	fmt.Print("Paste your API token: ")

	// Read token from user
	reader := bufio.NewReader(os.Stdin)
	token, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read token: %w", err)
	}
	token = strings.TrimSpace(token)

	if token == "" {
		return fmt.Errorf("token cannot be empty")
	}

	// Validate token by calling /api/user
	fmt.Println("\nValidating token...")
	user, err := validateToken(cfg.APIBaseURL, token)
	if err != nil {
		return fmt.Errorf("authentication failed: %w", err)
	}

	// Save token to config
	cfg.APIToken = token
	if err := cfg.Save(); err != nil {
		return fmt.Errorf("failed to save token: %w", err)
	}

	fmt.Printf("\n✓ Authenticated as %s (%s)\n", user.Name, user.Email)
	fmt.Println("\nYou can now use 'gears sync push' and 'gears sync pull' commands.")

	return nil
}

func validateToken(baseURL, token string) (*userResponse, error) {
	// Remove /v1 suffix if present and add /user endpoint
	apiURL := strings.Replace(baseURL, "/v1", "", 1) + "/user"

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("invalid token (status %d). Please check your token or create a new one", resp.StatusCode)
	}

	var user userResponse
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &user, nil
}

func openBrowser(url string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default: // linux
		cmd = exec.Command("xdg-open", url)
	}

	return cmd.Start()
}
