package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Config struct {
	WebListenAddr   string `json:"WebListenAddr"`
	IntervalMinutes int    `json:"IntervalMinutes"`
	BackupFolder    string `json:"BackupFolder"`
	Retention       int    `json:"Retention"`
	Sites           []Site `json:"Sites"`
}

type Site struct {
	Enabled bool   `json:"Enabled"`
	Name    string `json:"Name"`
	Url     string `json:"Url"`
}

// DefaultConfig returns a sensible default config.
// You can tweak these defaults later without breaking the JSON schema.
func DefaultConfig() Config {
	return Config{
		WebListenAddr:   "127.0.0.1:8123",
		IntervalMinutes: 5,
		BackupFolder:    defaultBackupFolder(),
		Retention:       30,
		Sites: []Site{
			{
				Enabled: true,
				Name:    "Example Site",
				Url:     "http://example.com/backup.zip",
			},
		},
	}
}

// LoadOrCreate loads config from path. If the file does not exist,
// it will create it with DefaultConfig() and return that default.
func LoadOrCreate(path string) (Config, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return Config{}, errors.New("config path is empty")
	}

	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			cfg := DefaultConfig()
			cfg.ValidateAndNormalize()

			if err := Save(path, cfg); err != nil {
				return Config{}, fmt.Errorf("failed to create default config at %q: %w", path, err)
			}
			return cfg, nil
		}
		return Config{}, fmt.Errorf("failed to read config %q: %w", path, err)
	}

	var cfg Config
	if err := json.Unmarshal(b, &cfg); err != nil {
		return Config{}, fmt.Errorf("failed to parse config %q: %w", path, err)
	}

	cfg.ValidateAndNormalize()
	return cfg, nil
}

// Save writes cfg to path as pretty-printed JSON. It creates the parent directory if needed.
func Save(path string, cfg Config) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return errors.New("config path is empty")
	}

	cfg.ValidateAndNormalize()

	// Ensure parent dir exists
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}
	b = append(b, '\n')

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return fmt.Errorf("failed to write temp config: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("failed to replace config: %w", err)
	}
	return nil
}

// ValidateAndNormalize applies minimal defaults/sanity.
// It does NOT hard-fail for most issues; it normalizes where possible.
// If you want strict validation later (e.g. error when URL is empty),
// we can add a separate ValidateStrict() that returns an error.
func (c *Config) ValidateAndNormalize() {
	// Defaults
	if c.IntervalMinutes < 0 {
		c.IntervalMinutes = 1
	}
	if c.Retention <= 0 {
		c.Retention = 30
	}
	c.BackupFolder = strings.TrimSpace(c.BackupFolder)
	if c.BackupFolder == "" {
		c.BackupFolder = defaultBackupFolder()
	}
	c.WebListenAddr = strings.TrimSpace(c.WebListenAddr)
	if c.WebListenAddr == "" {
		c.WebListenAddr = "127.0.0.1:8123"
	}

	// Normalize sites: trim whitespace
	out := make([]Site, 0, len(c.Sites))
	seen := map[string]struct{}{}
	for _, s := range c.Sites {
		s.Name = strings.TrimSpace(s.Name)
		s.Url = strings.TrimSpace(s.Url)

		// Skip totally empty entries (common when UI adds/removes rows)
		if s.Name == "" && s.Url == "" {
			continue
		}

		// If duplicate names exist, keep the first and drop later duplicates.
		// (Later we can instead auto-rename or return an error; this is the safest "don’t crash" behavior.)
		if s.Name != "" {
			if _, ok := seen[strings.ToLower(s.Name)]; ok {
				continue
			}
			seen[strings.ToLower(s.Name)] = struct{}{}
		}

		out = append(out, s)
	}
	c.Sites = out
}

func defaultBackupFolder() string {
	// Windows: %ProgramData%\httpBackupGo
	if pd := os.Getenv("ProgramData"); pd != "" {
		return filepath.Join(pd, "httpBackupGo", "Backups")
	}

	// Linux / macOS: ./Backups
	return "Backups"
}
