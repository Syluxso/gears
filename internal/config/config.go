package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
)

const ConfigFileName = ".gears/config.json"

// Config represents the .gears/config.json structure
type Config struct {
	WorkspaceID  string    `json:"workspace_id"`
	APIBaseURL   string    `json:"api_base_url"`
	APIToken     string    `json:"api_token,omitempty"`
	LastSync     time.Time `json:"last_sync,omitempty"`
}

// Load reads the config file from disk
func Load() (*Config, error) {
	data, err := os.ReadFile(ConfigFileName)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("config file not found. Run 'gears init' first")
		}
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	return &cfg, nil
}

// Save writes the config to disk
func (c *Config) Save() error {
	// Ensure .gears directory exists
	if err := os.MkdirAll(filepath.Dir(ConfigFileName), 0755); err != nil {
		return fmt.Errorf("failed to create .gears directory: %w", err)
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(ConfigFileName, data, 0644); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	return nil
}

// Exists checks if the config file exists
func Exists() bool {
	_, err := os.Stat(ConfigFileName)
	return err == nil
}

// GenerateWorkspaceID creates a new UUID v4 and returns it as a string
func GenerateWorkspaceID() string {
	return uuid.New().String()
}

// RequireAuth checks if the user is authenticated
func (c *Config) RequireAuth() error {
	if c.APIToken == "" {
		return fmt.Errorf("not authenticated. Run 'gears auth' first")
	}
	return nil
}

// DefaultAPIBaseURL returns the default API URL
func DefaultAPIBaseURL() string {
	// Can be overridden with environment variable
	if url := os.Getenv("GEARS_API_URL"); url != "" {
		return url
	}
	return "https://mygears.dev/api/v1"
}
