package events

import (
	"context"
	"sync"
)

// Type represents the category of an event.
type Type string

const (
	TypeStateChange   Type = "state_change"
	TypeAgentStart    Type = "agent_start"
	TypeAgentComplete Type = "agent_complete"
	TypeAgentError    Type = "agent_error"
	TypePipelineStart Type = "pipeline_start"
	TypePipelineEnd   Type = "pipeline_end"
)

// Event represents a system occurrence.
type Event struct {
	Type    Type
	Agent   string
	Payload map[string]interface{}
}

// Handler is called when an event is published.
type Handler func(ctx context.Context, event Event) error

// Bus manages event distribution.
type Bus interface {
	Publish(ctx context.Context, event Event) error
	Subscribe(t Type, h Handler) error
}

// InMemoryBus is a simple thread-safe implementation.
type InMemoryBus struct {
	mu       sync.RWMutex
	handlers map[Type][]Handler
}

func NewInMemoryBus() *InMemoryBus {
	return &InMemoryBus{
		handlers: make(map[Type][]Handler),
	}
}

func (b *InMemoryBus) Publish(ctx context.Context, event Event) error {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if hs, ok := b.handlers[event.Type]; ok {
		for _, h := range hs {
			if err := h(ctx, event); err != nil {
				return err
			}
		}
	}
	return nil
}

func (b *InMemoryBus) Subscribe(t Type, h Handler) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.handlers[t] = append(b.handlers[t], h)
	return nil
}
