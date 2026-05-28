package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/viper"
)

// Config represents the full autodev configuration.
type Config struct {
	Version     string              `yaml:"version"`
	LLM         LLMConfig           `yaml:"llm"`
	Models      []ModelEntry        `yaml:"models"`
	Agents      AgentsConfig        `yaml:"agents"`
	Memory      MemoryConfig        `yaml:"memory"`
	Session     SessionConfig       `yaml:"session"`
	MCP         MCPConfig           `yaml:"mcp"`
	Confirmation ConfirmationConfig `yaml:"confirmation"`
	Progress    ProgressConfig      `yaml:"progress"`
	Audit       AuditConfig         `yaml:"audit"`
	Secret      SecretConfig        `yaml:"secret"`
	Timeout     TimeoutConfig       `yaml:"timeout"`
	Sandbox     SandboxConfig       `yaml:"sandbox"`
	Diagnosis   DiagnosisConfig     `yaml:"diagnosis"`
	Analyzer    AnalyzerConfig      `yaml:"analyzer"`
	Output      OutputConfig        `yaml:"output"`
	Webhook     WebhookConfig       `yaml:"webhook"`
}

// LLMConfig defines the default LLM settings.
type LLMConfig struct {
	Provider string `yaml:"provider"`
	Model    string `yaml:"model"`
	APIKey   string `yaml:"api_key"`
	BaseURL  string `yaml:"base_url"`
}

// ModelEntry defines a specific model in the model pool.
type ModelEntry struct {
	Name     string `yaml:"name"`
	Provider string `yaml:"provider"`
	Model    string `yaml:"model"`
	APIKey   string `yaml:"api_key"`
	BaseURL  string `yaml:"base_url"`
}

// AgentsConfig defines agent-specific settings.
type AgentsConfig struct {
	Parser   AgentModelConfig `yaml:"parser"`
	Developer AgentModelConfig `yaml:"developer"`
	Tester   AgentModelConfig `yaml:"tester"`
	Checker  AgentModelConfig `yaml:"checker"`
	Recovery AgentModelConfig `yaml:"recovery"`
}

// AgentModelConfig defines per-agent model and generation settings.
type AgentModelConfig struct {
	Model       string  `yaml:"model"`
	Temperature float64 `yaml:"temperature"`
	MaxTokens   int     `yaml:"max_tokens"`
	Enabled     bool    `yaml:"enabled"`
}

// MemoryConfig defines memory system settings.
type MemoryConfig struct {
	Enabled bool   `yaml:"enabled"`
	DBPath  string `yaml:"db_path"`
}

// SessionConfig defines session storage settings.
type SessionConfig struct {
	Enabled          bool   `yaml:"enabled"`
	DBPath           string `yaml:"db_path"`
	AutoSave         bool   `yaml:"auto_save"`
	CheckpointInterval string `yaml:"checkpoint_interval"`
}

// MCPConfig defines MCP server settings.
type MCPConfig struct {
	Enabled bool              `yaml:"enabled"`
	Servers []MCPServerConfig `yaml:"servers"`
}

// MCPServerConfig defines a single MCP server.
type MCPServerConfig struct {
	Name    string   `yaml:"name"`
	Command string   `yaml:"command"`
	Args    []string `yaml:"args,omitempty"`
}

// ConfirmationConfig defines human confirmation settings.
type ConfirmationConfig struct {
	Enabled      bool `yaml:"enabled"`
	Interactive  bool `yaml:"interactive"`
	AutoApprove  bool `yaml:"auto_approve"`
}

// ProgressConfig defines progress reporting settings.
type ProgressConfig struct {
	Enabled bool   `yaml:"enabled"`
	Format  string `yaml:"format"`
	Verbose bool   `yaml:"verbose"`
}

// AuditConfig defines audit logging settings.
type AuditConfig struct {
	Enabled       bool   `yaml:"enabled"`
	DBPath        string `yaml:"db_path"`
	RetentionDays int    `yaml:"retention_days"`
}

// SecretConfig defines secret management settings.
type SecretConfig struct {
	Enabled   bool   `yaml:"enabled"`
	DBPath    string `yaml:"db_path"`
	Algorithm string `yaml:"algorithm"`
}

// TimeoutConfig defines timeout settings.
type TimeoutConfig struct {
	TaskMaxDuration    string `yaml:"task_max_duration"`
	StepMaxDuration    string `yaml:"step_max_duration"`
	LLMAPITimeout      string `yaml:"llm_api_timeout"`
	MCPToolTimeout     string `yaml:"mcp_tool_timeout"`
	TestExecutionTimeout string `yaml:"test_execution_timeout"`
}

// SandboxConfig defines sandbox execution settings.
type SandboxConfig struct {
	Enabled          bool     `yaml:"enabled"`
	MaxMemoryMB      int64    `yaml:"max_memory_mb"`
	MaxCpuTimeSeconds int     `yaml:"max_cpu_time_seconds"`
	NetworkAccess    bool     `yaml:"network_access"`
	AllowedCommands  []string `yaml:"allowed_commands"`
}

// DiagnosisConfig defines error diagnosis settings.
type DiagnosisConfig struct {
	Enabled bool `yaml:"enabled"`
}

// AnalyzerConfig defines project analyzer settings.
type AnalyzerConfig struct {
	Enabled    bool `yaml:"enabled"`
	AutoDetect bool `yaml:"auto_detect"`
}

// OutputConfig defines output formatting settings.
type OutputConfig struct {
	Format  string `yaml:"format"`
	LogLevel string `yaml:"log_level"`
	LogFile string `yaml:"log_file"`
}

// WebhookConfig defines webhook notification settings.
type WebhookConfig struct {
	Enabled bool     `yaml:"enabled"`
	URLs    []string `yaml:"urls"`
	Secret  string   `yaml:"secret"`
	Timeout string   `yaml:"timeout"`
	Retries int      `yaml:"retries"`
}

// LoadConfig loads configuration from files and environment variables.
func LoadConfig() (*Config, error) {
	v := viper.New()
	v.SetConfigName("autodev")
	v.SetConfigType("yaml")
	v.AddConfigPath(".")
	v.AddConfigPath("$HOME/.autodev")

	// Set defaults for required fields
	v.SetDefault("llm.provider", "openai")
	v.SetDefault("llm.model", "gpt-4o")
	v.SetDefault("memory.enabled", true)
	v.SetDefault("memory.db_path", "~/.autodev/memory.db")
	v.SetDefault("session.enabled", true)
	v.SetDefault("session.auto_save", true)
	v.SetDefault("mcp.enabled", true)
	v.SetDefault("confirmation.enabled", false)
	v.SetDefault("progress.enabled", true)
	v.SetDefault("progress.format", "terminal")
	v.SetDefault("audit.enabled", true)
	v.SetDefault("audit.db_path", "~/.autodev/audit.db")
	v.SetDefault("secret.enabled", true)
	v.SetDefault("timeout.llm_api_timeout", "60s")
	v.SetDefault("sandbox.enabled", false)
	v.SetDefault("diagnosis.enabled", true)
	v.SetDefault("analyzer.enabled", true)
	v.SetDefault("output.format", "console")
	v.SetDefault("output.log_level", "info")
	v.SetDefault("webhook.enabled", false)
	v.SetDefault("webhook.timeout", "10s")
	v.SetDefault("webhook.retries", 2)

	// Read config file (optional)
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, err
		}
	}

	// Allow environment variable overrides
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	// Resolve environment variables in API keys
	cfg.LLM.APIKey = resolveEnvVar(cfg.LLM.APIKey)
	cfg.LLM.Provider = resolveEnvVar(cfg.LLM.Provider)

	// Resolve home paths
	cfg.Memory.DBPath = resolveHomeDir(cfg.Memory.DBPath)
	cfg.Audit.DBPath = resolveHomeDir(cfg.Audit.DBPath)
	cfg.Secret.DBPath = resolveHomeDir(cfg.Secret.DBPath)

	return &cfg, nil
}

// GetModelByName looks up a model in the model pool.
func (c *Config) GetModelByName(name string) (*ModelEntry, error) {
	for _, m := range c.Models {
		if m.Name == name {
			return &m, nil
		}
	}
	return nil, fmt.Errorf("model %q not found in pool", name)
}

// IsAgentEnabled checks if a specific agent is enabled.
func (c *Config) IsAgentEnabled(role string) bool {
	switch strings.ToLower(role) {
	case "parser":
		return c.Agents.Parser.Enabled
	case "developer":
		return c.Agents.Developer.Enabled
	case "tester":
		return c.Agents.Tester.Enabled
	case "checker":
		return c.Agents.Checker.Enabled
	case "recovery":
		return c.Agents.Recovery.Enabled
	default:
		return true
	}
}

func resolveEnvVar(val string) string {
	if len(val) > 2 && val[0] == '$' {
		return os.Getenv(val[1:])
	}
	if len(val) > 3 && val[0] == '$' && val[1] == '{' {
		name := val[2 : len(val)-1]
		return os.Getenv(name)
	}
	return val
}

func resolveHomeDir(path string) string {
	if len(path) > 1 && path[:2] == "~/" {
		home, err := os.UserHomeDir()
		if err == nil {
			return home + path[1:]
		}
	}
	return path
}
