package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config holds all configuration for the dense-rag service.
type Config struct {
	Host          string   `yaml:"host"`
	Port          int      `yaml:"port"`
	TopK          int      `yaml:"topk"`
	WatchDirs     []string `yaml:"-"`
	Model         string   `yaml:"model"`
	ModelEndpoint string   `yaml:"model_endpoint"`
}

// rawConfig is used for YAML unmarshaling to support both watch_dir (string)
// and watch_dirs (array) formats.
type rawConfig struct {
	Host          string   `yaml:"host"`
	Port          int      `yaml:"port"`
	TopK          int      `yaml:"topk"`
	WatchDir      string   `yaml:"watch_dir"`
	WatchDirs     []string `yaml:"watch_dirs"`
	Model         string   `yaml:"model"`
	ModelEndpoint string   `yaml:"model_endpoint"`
}

// DefaultConfig returns a Config populated with default values.
func DefaultConfig() *Config {
	return &Config{
		Host:          "127.0.0.1",
		Port:          8123,
		TopK:          5,
		WatchDirs:     []string{"~/Documents"},
		Model:         "text-embedding-bge-m3",
		ModelEndpoint: "http://127.0.0.1:11434",
	}
}

// Load reads configuration from the given path and merges it with defaults.
// If path is empty, it defaults to ~/.dense_rag/config.yaml.
// If the file does not exist, defaults are returned without error.
func Load(path string) (*Config, error) {
	if path == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("get home directory: %w", err)
		}
		path = filepath.Join(home, ".dense_rag", "config.yaml")
	}

	cfg := DefaultConfig()

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			cfg.WatchDirs = expandTildeDirs(cfg.WatchDirs)
			return cfg, nil
		}
		return nil, fmt.Errorf("read config file: %w", err)
	}

	var raw rawConfig
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse config file: %w", err)
	}

	if raw.Host != "" {
		cfg.Host = raw.Host
	}
	if raw.Port != 0 {
		cfg.Port = raw.Port
	}
	if raw.TopK != 0 {
		cfg.TopK = raw.TopK
	}
	if raw.Model != "" {
		cfg.Model = raw.Model
	}
	if raw.ModelEndpoint != "" {
		cfg.ModelEndpoint = raw.ModelEndpoint
	}

	// watch_dirs (array) takes precedence over watch_dir (string).
	if len(raw.WatchDirs) > 0 {
		cfg.WatchDirs = raw.WatchDirs
	} else if raw.WatchDir != "" {
		cfg.WatchDirs = []string{raw.WatchDir}
	}

	cfg.WatchDirs = expandTildeDirs(cfg.WatchDirs)

	return cfg, nil
}

// Validate checks that the configuration values are acceptable.
func (c *Config) Validate() error {
	if c.Port < 1 || c.Port > 65535 {
		return fmt.Errorf("port must be between 1 and 65535, got %d", c.Port)
	}
	if len(c.WatchDirs) == 0 {
		return errors.New("watch_dirs must not be empty")
	}
	for _, d := range c.WatchDirs {
		if d == "" {
			return errors.New("watch_dirs entries must not be empty")
		}
	}
	if err := validateNoOverlap(c.WatchDirs); err != nil {
		return err
	}
	if c.Model == "" {
		return errors.New("model must not be empty")
	}
	if c.ModelEndpoint == "" {
		return errors.New("model_endpoint must not be empty")
	}
	return nil
}

// validateNoOverlap checks that no directory in dirs is an ancestor of another.
func validateNoOverlap(dirs []string) error {
	// Normalize all paths to absolute + clean form.
	abs := make([]string, len(dirs))
	for i, d := range dirs {
		a, err := filepath.Abs(d)
		if err != nil {
			return fmt.Errorf("resolve path %q: %w", d, err)
		}
		abs[i] = filepath.Clean(a)
	}

	for i := 0; i < len(abs); i++ {
		for j := i + 1; j < len(abs); j++ {
			if isAncestorOrEqual(abs[i], abs[j]) {
				return fmt.Errorf("watch_dirs overlap: %q contains %q", dirs[i], dirs[j])
			}
			if isAncestorOrEqual(abs[j], abs[i]) {
				return fmt.Errorf("watch_dirs overlap: %q contains %q", dirs[j], dirs[i])
			}
		}
	}
	return nil
}

// isAncestorOrEqual returns true if ancestor is a parent of (or equal to) child.
func isAncestorOrEqual(ancestor, child string) bool {
	if ancestor == child {
		return true
	}
	// Ensure ancestor ends with separator so "/mnt/c" doesn't match "/mnt/cdrom".
	prefix := ancestor + string(filepath.Separator)
	return strings.HasPrefix(child, prefix)
}

// expandTildeDirs applies tilde expansion to every entry in dirs.
func expandTildeDirs(dirs []string) []string {
	out := make([]string, len(dirs))
	for i, d := range dirs {
		out[i] = expandTilde(d)
	}
	return out
}

// expandTilde replaces a leading ~ in a path with the user's home directory.
func expandTilde(path string) string {
	if !strings.HasPrefix(path, "~") {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	return filepath.Join(home, path[1:])
}
