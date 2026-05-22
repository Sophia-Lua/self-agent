package core

// Config holds global configuration for the agent execution.
type Config struct {
	WorkDir   string
	DataDir   string
	MemoryDB  string
	Provider  string
	Model     string
}
