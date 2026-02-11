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
	Host          string `yaml:"host"`
	Port          int    `yaml:"port"`
	TopK          int    `yaml:"topk"`
	WatchDir      string `yaml:"watch_dir"`
	Model         string `yaml:"model"`
	ModelEndpoint string `yaml:"model_endpoint"`
}

// DefaultConfig returns a Config populated with default values.
func DefaultConfig() *Config {
	return &Config{
		Host:          "127.0.0.1",
		Port:          8123,
		TopK:          5,
		WatchDir:      "~/Documents",
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
			cfg.WatchDir = expandTilde(cfg.WatchDir)
			return cfg, nil
		}
		return nil, fmt.Errorf("read config file: %w", err)
	}

	// Unmarshal into a separate struct so we can detect which fields were
	// explicitly set in the file (non-zero values override defaults).
	var fileCfg Config
	if err := yaml.Unmarshal(data, &fileCfg); err != nil {
		return nil, fmt.Errorf("parse config file: %w", err)
	}

	if fileCfg.Host != "" {
		cfg.Host = fileCfg.Host
	}
	if fileCfg.Port != 0 {
		cfg.Port = fileCfg.Port
	}
	if fileCfg.TopK != 0 {
		cfg.TopK = fileCfg.TopK
	}
	if fileCfg.WatchDir != "" {
		cfg.WatchDir = fileCfg.WatchDir
	}
	if fileCfg.Model != "" {
		cfg.Model = fileCfg.Model
	}
	if fileCfg.ModelEndpoint != "" {
		cfg.ModelEndpoint = fileCfg.ModelEndpoint
	}

	cfg.WatchDir = expandTilde(cfg.WatchDir)

	return cfg, nil
}

// Validate checks that the configuration values are acceptable.
func (c *Config) Validate() error {
	if c.Port < 1 || c.Port > 65535 {
		return fmt.Errorf("port must be between 1 and 65535, got %d", c.Port)
	}
	if c.WatchDir == "" {
		return errors.New("watch_dir must not be empty")
	}
	if c.Model == "" {
		return errors.New("model must not be empty")
	}
	if c.ModelEndpoint == "" {
		return errors.New("model_endpoint must not be empty")
	}
	return nil
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
