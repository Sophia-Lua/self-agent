package events

import (
	"context"
	"sync"
	"testing"
)

func TestNewInMemoryBus(t *testing.T) {
	bus := NewInMemoryBus()
	if bus == nil {
		t.Fatal("bus is nil")
	}
}

func TestSubscribeAndPublish(t *testing.T) {
	bus := NewInMemoryBus()

	var received []Event
	err := bus.Subscribe(TypeStateChange, func(ctx context.Context, e Event) error {
		received = append(received, e)
		return nil
	})
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	event := Event{Type: TypeStateChange, Agent: "agent-1", Payload: map[string]interface{}{"state": "running"}}
	err = bus.Publish(context.Background(), event)
	if err != nil {
		t.Fatalf("Publish failed: %v", err)
	}

	if len(received) != 1 {
		t.Fatalf("received %d events, want 1", len(received))
	}
	if received[0].Agent != "agent-1" {
		t.Errorf("agent = %s", received[0].Agent)
	}
}

func TestPublishNoSubscribers(t *testing.T) {
	bus := NewInMemoryBus()

	err := bus.Publish(context.Background(), Event{Type: TypeStateChange})
	if err != nil {
		t.Errorf("publish with no subscribers should not error: %v", err)
	}
}

func TestPublishToUnknownType(t *testing.T) {
	bus := NewInMemoryBus()

	bus.Subscribe(TypeAgentStart, func(ctx context.Context, e Event) error {
		return nil
	})

	err := bus.Publish(context.Background(), Event{Type: Type("unknown")})
	if err != nil {
		t.Errorf("publish to unknown type should not error: %v", err)
	}
}

func TestMultipleSubscribersSameType(t *testing.T) {
	bus := NewInMemoryBus()

	var count1, count2 int
	bus.Subscribe(TypeAgentError, func(ctx context.Context, e Event) error {
		count1++
		return nil
	})
	bus.Subscribe(TypeAgentError, func(ctx context.Context, e Event) error {
		count2++
		return nil
	})

	bus.Publish(context.Background(), Event{Type: TypeAgentError})

	if count1 != 1 {
		t.Errorf("subscriber 1 received %d events", count1)
	}
	if count2 != 1 {
		t.Errorf("subscriber 2 received %d events", count2)
	}
}

func TestMultipleEventTypes(t *testing.T) {
	bus := NewInMemoryBus()

	var stateChanges, agentStarts int
	bus.Subscribe(TypeStateChange, func(ctx context.Context, e Event) error {
		stateChanges++
		return nil
	})
	bus.Subscribe(TypeAgentStart, func(ctx context.Context, e Event) error {
		agentStarts++
		return nil
	})

	bus.Publish(context.Background(), Event{Type: TypeStateChange})
	bus.Publish(context.Background(), Event{Type: TypeAgentStart})
	bus.Publish(context.Background(), Event{Type: TypeStateChange})

	if stateChanges != 2 {
		t.Errorf("state changes = %d, want 2", stateChanges)
	}
	if agentStarts != 1 {
		t.Errorf("agent starts = %d, want 1", agentStarts)
	}
}

func TestHandlerErrorStopsPublish(t *testing.T) {
	bus := NewInMemoryBus()

	callOrder := []string{}
	bus.Subscribe(TypeAgentError, func(ctx context.Context, e Event) error {
		callOrder = append(callOrder, "handler1")
		return nil
	})
	bus.Subscribe(TypeAgentError, func(ctx context.Context, e Event) error {
		callOrder = append(callOrder, "handler2")
		return testErr("handler failed")
	})
	bus.Subscribe(TypeAgentError, func(ctx context.Context, e Event) error {
		callOrder = append(callOrder, "handler3")
		return nil
	})

	err := bus.Publish(context.Background(), Event{Type: TypeAgentError})
	if err == nil {
		t.Error("expected error from handler")
	}

	if len(callOrder) != 2 {
		t.Errorf("call order = %v, want 2 handlers called", callOrder)
	}
	if callOrder[1] != "handler2" {
		t.Errorf("second handler should be handler2")
	}
}

func TestCancelledContext(t *testing.T) {
	bus := NewInMemoryBus()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	var called bool
	bus.Subscribe(TypeStateChange, func(ctx context.Context, e Event) error {
		called = true
		return ctx.Err()
	})

	err := bus.Publish(ctx, Event{Type: TypeStateChange})
	if err == nil {
		t.Error("expected error from cancelled context in handler")
	}
	if !called {
		t.Error("handler should have been called")
	}
}

func TestSubscribeConcurrent(t *testing.T) {
	bus := NewInMemoryBus()
	var wg sync.WaitGroup

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			bus.Subscribe(TypeAgentStart, func(ctx context.Context, e Event) error {
				return nil
			})
		}(i)
	}
	wg.Wait()

	event := Event{Type: TypeAgentStart}
	err := bus.Publish(context.Background(), event)
	if err != nil {
		t.Errorf("publish after concurrent subscribe failed: %v", err)
	}
}

func TestPublishConcurrent(t *testing.T) {
	bus := NewInMemoryBus()

	bus.Subscribe(TypeAgentStart, func(ctx context.Context, e Event) error {
		return nil
	})

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			bus.Publish(context.Background(), Event{Type: TypeAgentStart})
		}()
	}
	wg.Wait()
}

func TestEventTypes(t *testing.T) {
	if TypeStateChange != "state_change" {
		t.Errorf("TypeStateChange = %q", TypeStateChange)
	}
	if TypeAgentStart != "agent_start" {
		t.Errorf("TypeAgentStart = %q", TypeAgentStart)
	}
	if TypeAgentComplete != "agent_complete" {
		t.Errorf("TypeAgentComplete = %q", TypeAgentComplete)
	}
	if TypeAgentError != "agent_error" {
		t.Errorf("TypeAgentError = %q", TypeAgentError)
	}
}

func TestEventPayload(t *testing.T) {
	bus := NewInMemoryBus()

	var received Event
	bus.Subscribe(TypeAgentComplete, func(ctx context.Context, e Event) error {
		received = e
		return nil
	})

	payload := map[string]interface{}{
		"duration": 1200,
		"success":  true,
	}
	bus.Publish(context.Background(), Event{
		Type:    TypeAgentComplete,
		Agent:   "dev-1",
		Payload: payload,
	})

	if received.Payload["success"] != true {
		t.Errorf("payload success = %v", received.Payload["success"])
	}
}

type testErr string

func (e testErr) Error() string {
	return string(e)
}
