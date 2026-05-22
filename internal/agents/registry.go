package agents

import (
	"fmt"
	"sync"

	"autodev/internal/core"
)

// Registry stores available agents.
type Registry struct {
	mu     sync.RWMutex
	agents map[string]core.Agent
}

// NewRegistry initializes an empty agent registry.
func NewRegistry() *Registry {
	return &Registry{
		agents: make(map[string]core.Agent),
	}
}

// Register adds an agent to the registry.
func (r *Registry) Register(agent core.Agent) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.agents[agent.ID()]; exists {
		return fmt.Errorf("agent %s already registered", agent.ID())
	}

	r.agents[agent.ID()] = agent
	return nil
}

// Get retrieves an agent by ID.
func (r *Registry) Get(id string) (core.Agent, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	agent, exists := r.agents[id]
	if !exists {
		return nil, fmt.Errorf("agent %s not found", id)
	}
	return agent, nil
}

// List returns all registered agents.
func (r *Registry) List() []core.Agent {
	r.mu.RLock()
	defer r.mu.RUnlock()

	agents := make([]core.Agent, 0, len(r.agents))
	for _, a := range r.agents {
		agents = append(agents, a)
	}
	return agents
}
