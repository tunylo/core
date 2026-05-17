package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

type Config struct {
	DefaultTunnel *string                      `json:"default_tunnel,omitempty"`
	TunnelConfig  map[string]map[string]string `json:"tunnel_config,omitempty"`
}

// TunnelValue returns a stored config value for a given tunnel key and field name.
func (c *Config) TunnelValue(tunnelKey, field string) string {
	if c.TunnelConfig == nil {
		return ""
	}
	return c.TunnelConfig[tunnelKey][field]
}

// SetTunnelValue stores a config value for a given tunnel key and field name.
func (c *Config) SetTunnelValue(tunnelKey, field, value string) {
	if c.TunnelConfig == nil {
		c.TunnelConfig = make(map[string]map[string]string)
	}
	if c.TunnelConfig[tunnelKey] == nil {
		c.TunnelConfig[tunnelKey] = make(map[string]string)
	}
	c.TunnelConfig[tunnelKey][field] = value
}

func baseDir() (string, error) {
	if runtime.GOOS == "darwin" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, "Library", "Application Support", "tunylo"), nil
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "tunylo"), nil
}

func ConfigPath() (string, error) {
	dir, err := baseDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.json"), nil
}

func BinDir() (string, error) {
	dir, err := baseDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "bin"), nil
}

func NpmPackageDir(pkg string) (string, error) {
	dir, err := baseDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "npm", pkg, "node_modules", ".bin"), nil
}

func PidPath() (string, error) {
	dir, err := baseDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "tunylo.pid"), nil
}

func LogPath() (string, error) {
	dir, err := baseDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "tunylo.log"), nil
}

func NamedPidPath(name string) (string, error) {
	dir, err := baseDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "pids", name+".pid"), nil
}

func NamedLogPath(name string) (string, error) {
	dir, err := baseDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "logs", name+".log"), nil
}

func BaseDir() (string, error) {
	return baseDir()
}

func Load() (*Config, error) {
	path, err := ConfigPath()
	if err != nil {
		return &Config{}, nil
	}
	return loadFromPath(path)
}

func loadFromPath(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return &Config{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read config at %s: %w", path, err)
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config at %s: %w", path, err)
	}
	return &cfg, nil
}

func Save(cfg *Config) error {
	path, err := ConfigPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("failed to write config to %s: %w", path, err)
	}
	return nil
}
