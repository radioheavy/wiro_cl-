package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const legacyOutputDir = "./wiro-outputs"

// ProjectProfile contains local project metadata and API key.
type ProjectProfile struct {
	Name           string `json:"name"`
	APIKey         string `json:"apiKey"`
	AuthMethodHint string `json:"authMethodHint"`
}

// Preferences stores simple CLI defaults.
type Preferences struct {
	WatchDefault     bool   `json:"watchDefault"`
	OutputDirDefault string `json:"outputDirDefault"`
}

// Config is persisted under ~/.config/wiro/config.json.
type Config struct {
	DefaultProject string           `json:"defaultProject"`
	Projects       []ProjectProfile `json:"projects"`
	Preferences    Preferences      `json:"preferences"`
}

func defaultConfig() Config {
	return Config{
		Projects: []ProjectProfile{},
		Preferences: Preferences{
			WatchDefault:     true,
			OutputDirDefault: defaultOutputDir(),
		},
	}
}

func defaultOutputDir() string {
	return filepath.Join(defaultDownloadsDir(), "wiro-outputs")
}

func defaultDownloadsDir() string {
	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return "."
	}
	candidates := []string{
		filepath.Join(home, "Downloads"),
		filepath.Join(home, "İndirilenler"),
		filepath.Join(home, "indirilenler"),
	}
	for _, c := range candidates {
		if st, statErr := os.Stat(c); statErr == nil && st.IsDir() {
			return c
		}
	}
	// If none exists yet, default to common Downloads path.
	return candidates[0]
}

func configDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("get user config dir: %w", err)
	}
	return filepath.Join(base, "wiro"), nil
}

// ConfigPath returns the absolute config file path.
func ConfigPath() (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.json"), nil
}

// Load reads config from disk or returns defaults if missing.
func Load() (Config, error) {
	path, err := ConfigPath()
	if err != nil {
		return Config{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			cfg := defaultConfig()
			return cfg, nil
		}
		return Config{}, fmt.Errorf("read config: %w", err)
	}

	cfg := defaultConfig()
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config json: %w", err)
	}

	if cfg.Preferences.OutputDirDefault == "" || cfg.Preferences.OutputDirDefault == legacyOutputDir {
		cfg.Preferences.OutputDirDefault = defaultOutputDir()
	}
	return cfg, nil
}

// Save writes config atomically.
func Save(cfg Config) error {
	path, err := ConfigPath()
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	bytes, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, bytes, 0o600); err != nil {
		return fmt.Errorf("write tmp config: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("rename tmp config: %w", err)
	}
	return nil
}

// FindProject returns project by name or api key.
func (c Config) FindProject(nameOrKey string) *ProjectProfile {
	for i := range c.Projects {
		if c.Projects[i].Name == nameOrKey || c.Projects[i].APIKey == nameOrKey {
			return &c.Projects[i]
		}
	}
	return nil
}

// UpsertProject inserts/updates project profile by API key.
func (c *Config) UpsertProject(p ProjectProfile) {
	for i := range c.Projects {
		if c.Projects[i].APIKey == p.APIKey {
			if p.Name != "" {
				c.Projects[i].Name = p.Name
			}
			if p.AuthMethodHint != "" {
				c.Projects[i].AuthMethodHint = p.AuthMethodHint
			}
			return
		}
	}
	c.Projects = append(c.Projects, p)
}
