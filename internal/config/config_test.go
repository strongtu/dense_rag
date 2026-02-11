package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Host != "127.0.0.1" {
		t.Errorf("Host = %q, want %q", cfg.Host, "127.0.0.1")
	}
	if cfg.Port != 8123 {
		t.Errorf("Port = %d, want %d", cfg.Port, 8123)
	}
	if cfg.TopK != 5 {
		t.Errorf("TopK = %d, want %d", cfg.TopK, 5)
	}
	if cfg.WatchDir != "~/Documents" {
		t.Errorf("WatchDir = %q, want %q", cfg.WatchDir, "~/Documents")
	}
	if cfg.Model != "text-embedding-bge-m3" {
		t.Errorf("Model = %q, want %q", cfg.Model, "text-embedding-bge-m3")
	}
	if cfg.ModelEndpoint != "http://127.0.0.1:11434" {
		t.Errorf("ModelEndpoint = %q, want %q", cfg.ModelEndpoint, "http://127.0.0.1:11434")
	}
}

func TestLoad_NonExistentFile(t *testing.T) {
	cfg, err := Load("/tmp/dense_rag_test_nonexistent_config.yaml")
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	def := DefaultConfig()
	if cfg.Host != def.Host {
		t.Errorf("Host = %q, want default %q", cfg.Host, def.Host)
	}
	if cfg.Port != def.Port {
		t.Errorf("Port = %d, want default %d", cfg.Port, def.Port)
	}
	if cfg.TopK != def.TopK {
		t.Errorf("TopK = %d, want default %d", cfg.TopK, def.TopK)
	}
	if cfg.Model != def.Model {
		t.Errorf("Model = %q, want default %q", cfg.Model, def.Model)
	}
}

func TestLoad_ValidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	yamlContent := []byte("port: 9999\ntopk: 10\nmodel: custom-model\n")
	if err := os.WriteFile(path, yamlContent, 0644); err != nil {
		t.Fatalf("write test config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	// Overridden values
	if cfg.Port != 9999 {
		t.Errorf("Port = %d, want %d", cfg.Port, 9999)
	}
	if cfg.TopK != 10 {
		t.Errorf("TopK = %d, want %d", cfg.TopK, 10)
	}
	if cfg.Model != "custom-model" {
		t.Errorf("Model = %q, want %q", cfg.Model, "custom-model")
	}

	// Default values should be preserved
	if cfg.Host != "127.0.0.1" {
		t.Errorf("Host = %q, want default %q", cfg.Host, "127.0.0.1")
	}
	if cfg.ModelEndpoint != "http://127.0.0.1:11434" {
		t.Errorf("ModelEndpoint = %q, want default %q", cfg.ModelEndpoint, "http://127.0.0.1:11434")
	}
}

func TestLoad_TildeExpansion(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	yamlContent := []byte("watch_dir: \"~/MyDocs\"\n")
	if err := os.WriteFile(path, yamlContent, 0644); err != nil {
		t.Fatalf("write test config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("get home dir: %v", err)
	}

	expected := filepath.Join(home, "MyDocs")
	if cfg.WatchDir != expected {
		t.Errorf("WatchDir = %q, want %q", cfg.WatchDir, expected)
	}

	if strings.HasPrefix(cfg.WatchDir, "~") {
		t.Error("WatchDir still contains tilde after loading")
	}
}

func TestLoad_DefaultTildeExpansion(t *testing.T) {
	// Load from a non-existent file to get defaults with tilde expanded
	cfg, err := Load("/tmp/dense_rag_test_nonexistent_config.yaml")
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if strings.HasPrefix(cfg.WatchDir, "~") {
		t.Error("WatchDir still contains tilde after loading defaults")
	}

	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("get home dir: %v", err)
	}

	expected := filepath.Join(home, "Documents")
	if cfg.WatchDir != expected {
		t.Errorf("WatchDir = %q, want %q", cfg.WatchDir, expected)
	}
}

func TestValidate_InvalidPort(t *testing.T) {
	tests := []struct {
		name string
		port int
	}{
		{"zero", 0},
		{"negative", -1},
		{"too_large", 65536},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig()
			cfg.Port = tt.port
			if err := cfg.Validate(); err == nil {
				t.Errorf("Validate() should return error for port %d", tt.port)
			}
		})
	}
}

func TestValidate_Defaults(t *testing.T) {
	cfg := DefaultConfig()
	if err := cfg.Validate(); err != nil {
		t.Errorf("Validate() returned error for defaults: %v", err)
	}
}

func TestValidate_EmptyWatchDir(t *testing.T) {
	cfg := DefaultConfig()
	cfg.WatchDir = ""
	if err := cfg.Validate(); err == nil {
		t.Error("Validate() should return error for empty WatchDir")
	}
}

func TestValidate_EmptyModel(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Model = ""
	if err := cfg.Validate(); err == nil {
		t.Error("Validate() should return error for empty Model")
	}
}

func TestValidate_EmptyModelEndpoint(t *testing.T) {
	cfg := DefaultConfig()
	cfg.ModelEndpoint = ""
	if err := cfg.Validate(); err == nil {
		t.Error("Validate() should return error for empty ModelEndpoint")
	}
}
