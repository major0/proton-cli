package api

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// ShareConfig controls per-share caching policy. All toggles default
// to false (strictest encrypted-data-handling compliance).
type ShareConfig struct {
	DirentCacheEnabled   bool `yaml:"dirent_cache_enabled"`
	MetadataCacheEnabled bool `yaml:"metadata_cache_enabled"`
	DiskCacheEnabled     bool `yaml:"disk_cache_enabled"`
}

// Config holds application-level settings loaded from YAML.
type Config struct {
	Shares   map[string]ShareConfig `yaml:"shares,omitempty"`
	Defaults map[string]string      `yaml:"defaults,omitempty"`
}

// DefaultConfig returns a Config with all defaults (empty maps, caching off).
func DefaultConfig() *Config {
	return &Config{
		Shares:   make(map[string]ShareConfig),
		Defaults: make(map[string]string),
	}
}

// DefaultAccount returns the configured default account for a service,
// or "default" when not configured.
func (c *Config) DefaultAccount(service string) string {
	if c.Defaults != nil {
		if acct, ok := c.Defaults[service]; ok && acct != "" {
			return acct
		}
	}
	return "default"
}

// LoadConfig reads a YAML config file. Returns DefaultConfig if the file
// does not exist. Returns an error only for I/O or parse failures.
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path) //nolint:gosec // path from user config, not tainted input
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return DefaultConfig(), nil
		}
		return nil, fmt.Errorf("config: read %s: %w", path, err)
	}

	cfg := DefaultConfig()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("config: parse %s: %w", path, err)
	}

	// Ensure maps are initialized even if YAML had empty sections.
	if cfg.Shares == nil {
		cfg.Shares = make(map[string]ShareConfig)
	}
	if cfg.Defaults == nil {
		cfg.Defaults = make(map[string]string)
	}

	return cfg, nil
}

// SaveConfig writes the config to path as YAML. Creates parent directories
// and uses atomic write (temp file + rename) to prevent corruption.
func SaveConfig(path string, cfg *Config) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("config: mkdir %s: %w", dir, err)
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("config: marshal: %w", err)
	}

	// Atomic write: temp file in same directory, then rename.
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0600); err != nil {
		return fmt.Errorf("config: write %s: %w", tmp, err)
	}

	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("config: rename %s: %w", path, err)
	}

	return nil
}
