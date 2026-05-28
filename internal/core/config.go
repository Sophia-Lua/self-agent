package core

// Config holds global configuration for the agent execution.
type Config struct {
	WorkDir  string `yaml:"work_dir"`
	DataDir  string `yaml:"data_dir"`
	MemoryDB string `yaml:"memory_db"`
	Provider string `yaml:"provider"`
	Model    string `yaml:"model"`
}

// PRConfig holds configuration for automatic PR creation after pipeline completion.
type PRConfig struct {
	Enabled      bool     `yaml:"enabled"`
	Platform     string   `yaml:"platform"`
	Token        string   `yaml:"token"`
	Owner        string   `yaml:"owner"`
	Repo         string   `yaml:"repo"`
	TargetBranch string   `yaml:"target_branch"`
	Labels       []string `yaml:"labels,omitempty"`
	Reviewers    []string `yaml:"reviewers,omitempty"`
	Draft        bool     `yaml:"draft,omitempty"`
}

