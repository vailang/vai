package config

// LibConfig holds library identity and source layout.
type LibConfig struct {
	Name    string `toml:"name"`
	Prompts string `toml:"prompts"`
}

// LLMConfig defines the configuration for an LLM agent (Planner or Executor).
// Provider and Model are not required when BaseURL is set.
// Schema selects the wire format ("openai", "anthropic", "gemini") for custom providers.
type LLMConfig struct {
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

	// Planner is the LLM configuration for the architect agent.
	Planner LLMConfig `toml:"planner"`

	// Executor is the LLM configuration for the code generator agent.
	Executor LLMConfig `toml:"executor"`

	// Debug is the debug configuration for the target language.
	Debug DebugConfig `toml:"debug"`

	// Vars holds variables for match/case resolution.
	Vars map[string]string `toml:"vars"`
}
