package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// ErrNoConfig is returned when no vai.toml is found walking upward.
var ErrNoConfig = errors.New("vai.toml not found")

// FindConfig walks upward from startDir looking for vai.toml.
// Returns the absolute path to the first vai.toml found.
func FindConfig(startDir string) (string, error) {
	dir, err := filepath.Abs(startDir)
	if err != nil {
		return "", err
	}
	for {
		candidate := filepath.Join(dir, "vai.toml")
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", ErrNoConfig
		}
		dir = parent
	}
}

// LoadConfig reads and decodes a vai.toml file into a Config.
func LoadConfig(path string) (*Config, error) {
	var cfg Config
	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	return &cfg, nil
}
