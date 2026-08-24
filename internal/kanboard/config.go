package kanboard

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Kanboard settings live alongside the other gearbox config, but the token is
// deliberately kept out of config.json: in most workspaces .gears/ is committed,
// so anything written there ends up in git.
const (
	configFile      = ".gears/.gearbox/config.json"
	localConfigFile = ".gears/.gearbox/config-local.json"
	tokenEnvVar     = "KANBOARD_API_TOKEN"
	defaultUsername = "jsonrpc"
)

// Settings is the resolved Kanboard connection config.
type Settings struct {
	URL      string
	Username string
	Token    string
}

// fileConfig mirrors only the keys we care about so we never clobber unrelated
// config written by other commands.
type fileConfig struct {
	Kanboard *struct {
		URL      string `json:"url,omitempty"`
		Username string `json:"username,omitempty"`
		APIToken string `json:"api_token,omitempty"`
	} `json:"kanboard,omitempty"`
}

// LoadSettings resolves connection settings. The token is looked up in
// KANBOARD_API_TOKEN first, then config-local.json, then config.json.
func LoadSettings() (*Settings, error) {
	s := &Settings{Username: defaultUsername}

	// Non-secret settings.
	if cfg, err := readFileConfig(configFile); err == nil && cfg.Kanboard != nil {
		s.URL = cfg.Kanboard.URL
		if cfg.Kanboard.Username != "" {
			s.Username = cfg.Kanboard.Username
		}
		s.Token = cfg.Kanboard.APIToken
	}

	// config-local.json overrides, and is the intended home for the token.
	if cfg, err := readFileConfig(localConfigFile); err == nil && cfg.Kanboard != nil {
		if cfg.Kanboard.URL != "" {
			s.URL = cfg.Kanboard.URL
		}
		if cfg.Kanboard.Username != "" {
			s.Username = cfg.Kanboard.Username
		}
		if cfg.Kanboard.APIToken != "" {
			s.Token = cfg.Kanboard.APIToken
		}
	}

	// Environment wins over everything.
	if env := strings.TrimSpace(os.Getenv(tokenEnvVar)); env != "" {
		s.Token = env
	}

	if s.URL == "" {
		return nil, fmt.Errorf("no Kanboard URL configured\n\n  Set one with:\n    gears kan --url=https://your-board.example.com")
	}
	if s.Token == "" {
		return nil, fmt.Errorf("no Kanboard API token configured\n\n  Set one with:\n    gears kan --set-api-key        (prompts, keeps it out of shell history)\n\n  Or export %s", tokenEnvVar)
	}

	return s, nil
}

func readFileConfig(path string) (*fileConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg fileConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// SaveURL writes the board URL to config.json (safe to commit).
func SaveURL(rawURL string) error {
	return mutateConfig(configFile, func(m map[string]any) {
		kb := kanboardSection(m)
		kb["url"] = strings.TrimRight(strings.TrimSpace(rawURL), "/")
	})
}

// SaveUsername writes the API username to config.json (safe to commit).
func SaveUsername(username string) error {
	return mutateConfig(configFile, func(m map[string]any) {
		kb := kanboardSection(m)
		kb["username"] = strings.TrimSpace(username)
	})
}

// SaveToken writes the API token to config-local.json, never config.json,
// and ensures the file is ignored by git.
func SaveToken(token string) error {
	if err := mutateConfig(localConfigFile, func(m map[string]any) {
		kb := kanboardSection(m)
		kb["api_token"] = strings.TrimSpace(token)
	}); err != nil {
		return err
	}
	return ensureLocalConfigIgnored()
}

func kanboardSection(m map[string]any) map[string]any {
	existing, ok := m["kanboard"].(map[string]any)
	if !ok {
		existing = map[string]any{}
		m["kanboard"] = existing
	}
	return existing
}

// mutateConfig does a read-modify-write against a JSON file, preserving any
// keys written by other commands.
func mutateConfig(path string, fn func(map[string]any)) error {
	m := map[string]any{}
	if data, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(data, &m); err != nil {
			return fmt.Errorf("failed to parse %s: %w", path, err)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("failed to read %s: %w", path, err)
	}

	fn(m)

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to encode config: %w", err)
	}

	// 0600 on the file that holds the token.
	mode := os.FileMode(0644)
	if path == localConfigFile {
		mode = 0600
	}
	if err := os.WriteFile(path, data, mode); err != nil {
		return fmt.Errorf("failed to write %s: %w", path, err)
	}
	return nil
}

// ensureLocalConfigIgnored appends config-local.json to .gears/.gitignore if it
// is not already covered. Best effort: a failure here is reported by the caller
// but does not invalidate the saved token.
func ensureLocalConfigIgnored() error {
	ignorePath := filepath.Join(".gears", ".gitignore")
	entry := ".gearbox/config-local.json"

	data, err := os.ReadFile(ignorePath)
	if err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			if strings.TrimSpace(line) == entry {
				return nil
			}
		}
		body := string(data)
		if !strings.HasSuffix(body, "\n") {
			body += "\n"
		}
		body += entry + "\n"
		return os.WriteFile(ignorePath, []byte(body), 0644)
	}
	if !os.IsNotExist(err) {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(ignorePath), 0755); err != nil {
		return err
	}
	content := "# Local-only gears config (API tokens) - never commit\n" + entry + "\n"
	return os.WriteFile(ignorePath, []byte(content), 0644)
}

// Redact replaces the token wherever it appears, so it cannot leak through
// error messages or verbose output.
func (s *Settings) Redact(text string) string {
	if s == nil || s.Token == "" {
		return text
	}
	return strings.ReplaceAll(text, s.Token, "***REDACTED***")
}
