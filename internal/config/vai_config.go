package config

// LibConfig holds library identity and source layout.
type LibConfig struct {
	Name    string `toml:"name"`
	Prompts string `toml:"prompts"`
}

// LLMConfig defines the configuration for an LLM agent (Planner or Executor).
// Provider and Model are not required when BaseURL is set.
type LLMConfig struct {
	Provider             string `toml:"provider"`
	Model                string `toml:"model"`
	BaseURL              string `toml:"base_url"`
	EnvTokenVariableName string `toml:"env_token_variable_name"`
	MaxTokens            int    `toml:"max_tokens"`
	MaxRetries           int    `toml:"max_retries"`
	DelayRetrySeconds    int    `toml:"delay_retry_seconds"`
}

// DebugToolConfig defines a single debug tool for a target language.
// Cmd is the command to execute, Format is the expected output format (e.g. "json", "stdout").
type DebugToolConfig struct {
	Cmd    string `toml:"cmd"`
	Format string `toml:"format"`
}

// DebugConfig holds debug configuration for the target language.
type DebugConfig struct {
	Tools []DebugToolConfig `toml:"tools"`
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
