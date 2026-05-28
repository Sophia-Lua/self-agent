package llm

import (
	"fmt"
	"time"
)

// ProviderConfig defines the parameters needed to create an LLM Provider.
type ProviderConfig struct {
	Provider       string `yaml:"provider"`
	Model          string `yaml:"model"`
	APIKey         string `yaml:"api_key"`
	BaseURL        string `yaml:"base_url"`
	EnableCache    bool   `yaml:"enable_cache"`
	MaxCacheSize   int    `yaml:"max_cache_size"`
	EnableRateLimit bool  `yaml:"enable_rate_limit"`
	MaxRetries     int    `yaml:"max_retries"`
	BaseDelayMs    int    `yaml:"base_delay_ms"`
}

// NewProvider creates a Provider based on the given configuration.
// Automatically wraps with caching and rate limiting if enabled in config.
func NewProvider(cfg ProviderConfig) (Provider, error) {
	var inner Provider

	switch cfg.Provider {
	case "openai", "azure", "":
		baseURL := cfg.BaseURL
		if baseURL == "" {
			baseURL = "https://api.openai.com/v1"
		}
		inner = &OpenAIProvider{
			BaseURL: baseURL,
			APIKey:  cfg.APIKey,
			Model:   cfg.Model,
		}
	case "claude", "anthropic":
		inner = NewClaudeProvider(cfg.APIKey, cfg.Model)
	case "ollama", "local":
		inner = NewOllamaProvider(cfg.BaseURL, cfg.Model)
	case "mock":
		inner = &MockProvider{}
	default:
		return nil, fmt.Errorf("unsupported provider: %s", cfg.Provider)
	}

	// Wrap with caching if enabled
	if cfg.EnableCache {
		maxSize := cfg.MaxCacheSize
		if maxSize <= 0 {
			maxSize = 1000
		}
		inner = NewCacheProvider(inner, maxSize)
	}

	// Wrap with rate limiting if enabled
	if cfg.EnableRateLimit {
		maxRetries := cfg.MaxRetries
		if maxRetries <= 0 {
			maxRetries = 3
		}
		baseDelay := cfg.BaseDelayMs
		if baseDelay <= 0 {
			baseDelay = 1000
		}
		return WithRateLimit(inner, maxRetries, time.Duration(baseDelay)*time.Millisecond), nil
	}

	return inner, nil
}
