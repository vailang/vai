package provider

import (
	"fmt"

	"github.com/vailang/vai/internal/config"
)

// New creates a Provider from an LLMConfig.
// For custom providers, the Schema field selects the wire format.
func New(cfg config.LLMConfig) (Provider, error) {
	switch cfg.Provider {
	case "anthropic":
		return newAnthropic(cfg), nil
	case "openai":
		return newOpenAI(cfg), nil
	case "gemini":
		return newGemini(cfg), nil
	case "custom", "":
		if cfg.BaseURL != "" {
			return newFromSchema(cfg)
		}
		if cfg.Provider == "" {
			return nil, fmt.Errorf("provider or base_url required")
		}
		return nil, fmt.Errorf("custom provider requires base_url")
	default:
		if cfg.BaseURL != "" {
			return newFromSchema(cfg)
		}
		return nil, fmt.Errorf("unknown provider: %s", cfg.Provider)
	}
}

// newFromSchema dispatches to a provider constructor based on the Schema field.
func newFromSchema(cfg config.LLMConfig) (Provider, error) {
	switch cfg.Schema {
	case "anthropic":
		return newAnthropic(cfg), nil
	case "gemini":
		return newGemini(cfg), nil
	case "openai", "":
		return newOpenAI(cfg), nil
	default:
		return nil, fmt.Errorf("unknown schema: %s (use openai, anthropic, or gemini)", cfg.Schema)
	}
}
