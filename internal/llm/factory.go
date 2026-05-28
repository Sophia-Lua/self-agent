package llm

import "fmt"

// ProviderConfig defines the parameters needed to create an LLM Provider.
type ProviderConfig struct {
	Provider string `yaml:"provider"`
	Model    string `yaml:"model"`
	APIKey   string `yaml:"api_key"`
	BaseURL  string `yaml:"base_url"`
}

// NewProvider creates a Provider based on the given configuration.
func NewProvider(cfg ProviderConfig) (Provider, error) {
	switch cfg.Provider {
	case "openai", "azure", "":
		baseURL := cfg.BaseURL
		if baseURL == "" {
			baseURL = "https://api.openai.com/v1"
		}
		return &OpenAIProvider{
			BaseURL: baseURL,
			APIKey:  cfg.APIKey,
			Model:   cfg.Model,
		}, nil
	case "claude", "anthropic":
		return NewClaudeProvider(cfg.APIKey, cfg.Model), nil
	case "ollama", "local":
		return NewOllamaProvider(cfg.BaseURL, cfg.Model), nil
	case "mock":
		return &MockProvider{}, nil
	default:
		return nil, fmt.Errorf("unsupported provider: %s", cfg.Provider)
	}
}
