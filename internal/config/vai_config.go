package config

import "fmt"

// LibConfig holds library identity and source layout.
type LibConfig struct {
	Name    string `toml:"name"`
	Prompts string `toml:"prompts"`
}

// LLMConfig defines the configuration for an LLM provider.
// Provider and Model are not required when BaseURL is set.
// Schema selects the wire format ("openai", "anthropic", "gemini") for custom providers.
type LLMConfig struct {
	Name                 string `toml:"name"`     // optional human label
	Role                 string `toml:"role"`     // "plan", "code", or "" (no default role)
	Provider             string `toml:"provider"`
	Schema               string `toml:"schema"`
	Model                string `toml:"model"`
	BaseURL              string `toml:"base_url"`
	EnvTokenVariableName string `toml:"env_token_variable_name"`
	MaxTokens            int    `toml:"max_tokens"`
	MaxRetries           int    `toml:"max_retries"`
	DelayRetrySeconds    int    `toml:"delay_retry_seconds"`
}

// DebugLangConfig defines compile-check and tools for one target language.
type DebugLangConfig struct {
	CompileCheck string   `toml:"compile_check"`
	Format       string   `toml:"format"` // "json" for structured output, empty for raw text
	Tools        []string `toml:"tools"`
}

// DebugConfig holds debug configuration keyed by language.
type DebugConfig struct {
	MaxAttempts int                        `toml:"max_attempts"`
	Languages   map[string]DebugLangConfig `toml:"languages"`
}

// Config is the main Vai configuration, loaded from vai.toml.
type Config struct {
	// Lib holds library identity and source layout.
	Lib LibConfig `toml:"lib"`

	// LLMs is the list of configured LLM providers.
	LLMs []LLMConfig `toml:"llm"`

	// Debug is the debug configuration for the target language.
	Debug DebugConfig `toml:"debug"`

	// Vars holds variables for match/case resolution.
	Vars map[string]string `toml:"vars"`
}

// ValidateRole checks that a role string is valid ("plan", "code", or empty).
func ValidateRole(role string) error {
	if role != "" && role != "plan" && role != "code" {
		return fmt.Errorf("role must be \"plan\", \"code\", or empty; got %q", role)
	}
	return nil
}

// PlannerConfig returns the LLM config with role "plan".
// Returns an error if none or more than one have role "plan".
func (c *Config) PlannerConfig() (LLMConfig, error) {
	return c.byRole("plan")
}

// ExecutorConfig returns the LLM config with role "code".
// Returns an error if none or more than one have role "code".
func (c *Config) ExecutorConfig() (LLMConfig, error) {
	return c.byRole("code")
}

func (c *Config) byRole(role string) (LLMConfig, error) {
	var matches []LLMConfig
	for _, l := range c.LLMs {
		if l.Role == role {
			matches = append(matches, l)
		}
	}
	switch len(matches) {
	case 0:
		return LLMConfig{}, fmt.Errorf("no LLM with role %q configured in vai.toml", role)
	case 1:
		return matches[0], nil
	default:
		return LLMConfig{}, fmt.Errorf("multiple LLMs with role %q configured in vai.toml (expected exactly one)", role)
	}
}
