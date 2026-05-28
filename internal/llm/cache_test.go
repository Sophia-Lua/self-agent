package llm

import (
	"context"
	"sync/atomic"
	"testing"

	"autodev/internal/core"
)

func TestCacheProviderReturnsCachedResult(t *testing.T) {
	callCount := atomic.Int64{}
	inner := &testCacheProvider{
		chatFunc: func(ctx context.Context, messages []core.Message, tools []core.Tool) (*core.AgentOutput, error) {
			callCount.Add(1)
			return &core.AgentOutput{Content: "cached response"}, nil
		},
	}

	cache := NewCacheProvider(inner, 100)
	messages := []core.Message{{Role: "user", Content: "hello"}}

	for i := 0; i < 5; i++ {
		result, err := cache.Chat(context.Background(), messages, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Content != "cached response" {
			t.Errorf("expected cached response, got: %s", result.Content)
		}
	}

	if callCount.Load() != 1 {
		t.Errorf("expected 1 inner call (cached 4 times), got: %d", callCount.Load())
	}
}

func TestCacheProviderDifferentInputs(t *testing.T) {
	callCount := atomic.Int64{}
	inner := &testCacheProvider{
		chatFunc: func(ctx context.Context, messages []core.Message, tools []core.Tool) (*core.AgentOutput, error) {
			callCount.Add(1)
			return &core.AgentOutput{Content: "unique response", ToolCalls: nil}, nil
		},
	}

	cache := NewCacheProvider(inner, 100)

	msg1 := []core.Message{{Role: "user", Content: "hello"}}
	msg2 := []core.Message{{Role: "user", Content: "world"}}

	_, _ = cache.Chat(context.Background(), msg1, nil)
	_, _ = cache.Chat(context.Background(), msg2, nil)
	_, _ = cache.Chat(context.Background(), msg1, nil)

	if callCount.Load() != 2 {
		t.Errorf("expected 2 inner calls (different messages), got: %d", callCount.Load())
	}
}

func TestCacheProviderMaxSizeEviction(t *testing.T) {
	inner := &testCacheProvider{
		chatFunc: func(ctx context.Context, messages []core.Message, tools []core.Tool) (*core.AgentOutput, error) {
			return &core.AgentOutput{Content: "ok"}, nil
		},
	}

	cache := NewCacheProvider(inner, 3)

	for i := 0; i < 5; i++ {
		msg := []core.Message{{Role: "user", Content: string(rune('A' + i))}}
		_, _ = cache.Chat(context.Background(), msg, nil)
	}

	if cache.CacheSize() > 3 {
		t.Errorf("cache should not exceed maxSize 3, got: %d", cache.CacheSize())
	}
}

func TestCacheProviderClear(t *testing.T) {
	inner := &testCacheProvider{
		chatFunc: func(ctx context.Context, messages []core.Message, tools []core.Tool) (*core.AgentOutput, error) {
			return &core.AgentOutput{Content: "ok"}, nil
		},
	}

	cache := NewCacheProvider(inner, 100)
	msg := []core.Message{{Role: "user", Content: "test"}}
	_, _ = cache.Chat(context.Background(), msg, nil)

	if cache.CacheSize() != 1 {
		t.Errorf("expected 1 cached entry, got: %d", cache.CacheSize())
	}

	cache.Clear()
	if cache.CacheSize() != 0 {
		t.Errorf("expected 0 cached entries after Clear, got: %d", cache.CacheSize())
	}
}

func TestCacheProviderPropagatesErrors(t *testing.T) {
	inner := &testCacheProvider{
		chatFunc: func(ctx context.Context, messages []core.Message, tools []core.Tool) (*core.AgentOutput, error) {
			return nil, context.Canceled
		},
	}

	cache := NewCacheProvider(inner, 100)
	_, err := cache.Chat(context.Background(), nil, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err != context.Canceled {
		t.Errorf("expected context.Canceled, got: %v", err)
	}
}

func TestCacheProviderDelegatesNameAndCapabilities(t *testing.T) {
	inner := &testCacheProvider{
		nameFunc: func() string { return "test" },
		capsFunc: func() core.Capabilities { return core.Capabilities{MaxTokens: 4096} },
	}

	cache := NewCacheProvider(inner, 100)

	if cache.Name() != "test (cached)" {
		t.Errorf("expected 'test (cached)', got: %s", cache.Name())
	}

	caps := cache.Capabilities()
	if caps.MaxTokens != 4096 {
		t.Errorf("expected MaxTokens 4096, got: %d", caps.MaxTokens)
	}
}

type testCacheProvider struct {
	chatFunc func(ctx context.Context, messages []core.Message, tools []core.Tool) (*core.AgentOutput, error)
	nameFunc func() string
	capsFunc func() core.Capabilities
}

func (p *testCacheProvider) Chat(ctx context.Context, messages []core.Message, tools []core.Tool) (*core.AgentOutput, error) {
	return p.chatFunc(ctx, messages, tools)
}

func (p *testCacheProvider) Name() string {
	if p.nameFunc != nil {
		return p.nameFunc()
	}
	return "test"
}

func (p *testCacheProvider) Capabilities() core.Capabilities {
	if p.capsFunc != nil {
		return p.capsFunc()
	}
	return core.Capabilities{}
}
