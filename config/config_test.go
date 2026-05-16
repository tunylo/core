package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadMissingFile(t *testing.T) {
	cfg, err := loadFromPath(filepath.Join(t.TempDir(), "config.json"))
	if err != nil {
		t.Fatalf("Load with no config file should not error, got: %v", err)
	}
	if cfg.DefaultTunnel != nil {
		t.Fatal("expected nil DefaultTunnel on fresh config")
	}
}

func TestSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	tunnel := "cloudflare"
	cfg := &Config{DefaultTunnel: &tunnel}

	data, _ := json.Marshal(cfg)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}

	var loaded Config
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &loaded); err != nil {
		t.Fatal(err)
	}
	if loaded.DefaultTunnel == nil || *loaded.DefaultTunnel != tunnel {
		t.Fatalf("expected DefaultTunnel %q, got %v", tunnel, loaded.DefaultTunnel)
	}
}

func TestLoadInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	os.WriteFile(path, []byte("{invalid}"), 0o600)

	var cfg Config
	if err := json.Unmarshal([]byte("{invalid}"), &cfg); err == nil {
		t.Fatal("expected error parsing invalid JSON")
	}
}
