package core

// Config holds global configuration for the agent execution.
type Config struct {
	WorkDir  string `yaml:"work_dir"`
	DataDir  string `yaml:"data_dir"`
	MemoryDB string `yaml:"memory_db"`
	Provider string `yaml:"provider"`
	Model    string `yaml:"model"`
}
