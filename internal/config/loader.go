package config

import (
	"bytes"
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

// rawConfig is used for TOML decoding to support both the new [[llm]]
// format and the legacy [planner]/[executor] format.
type rawConfig struct {
	Lib      LibConfig         `toml:"lib"`
	LLMs     []LLMConfig       `toml:"llm"`
	Planner  LLMConfig         `toml:"planner"`
	Executor LLMConfig         `toml:"executor"`
	Debug    DebugConfig       `toml:"debug"`
	Vars     map[string]string `toml:"vars"`
}

func isLLMSet(l LLMConfig) bool {
	return l.Provider != "" || l.BaseURL != ""
}

// LoadConfig reads and decodes a vai.toml file into a Config.
// Supports both the new [[llm]] format and the legacy [planner]/[executor] format.
func LoadConfig(path string) (*Config, error) {
	var raw rawConfig
	if _, err := toml.DecodeFile(path, &raw); err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}

	cfg := &Config{
		Lib:   raw.Lib,
		LLMs:  raw.LLMs,
		Debug: raw.Debug,
		Vars:  raw.Vars,
	}

	// Auto-migrate legacy [planner]/[executor] when no [[llm]] entries exist.
	if len(cfg.LLMs) == 0 {
		if isLLMSet(raw.Planner) {
			raw.Planner.Role = "plan"
			cfg.LLMs = append(cfg.LLMs, raw.Planner)
		}
		if isLLMSet(raw.Executor) {
			raw.Executor.Role = "code"
			cfg.LLMs = append(cfg.LLMs, raw.Executor)
		}
	}

	return cfg, nil
}

// SaveConfig writes a Config to a vai.toml file in the new [[llm]] format.
func SaveConfig(path string, cfg *Config) error {
	var buf bytes.Buffer
	enc := toml.NewEncoder(&buf)
	if err := enc.Encode(cfg); err != nil {
		return fmt.Errorf("encoding config: %w", err)
	}
	return os.WriteFile(path, buf.Bytes(), 0644)
}
