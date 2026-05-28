package llm

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"autodev/internal/core"
)

func TestRateLimitedProviderRetriesOnRateLimit(t *testing.T) {
	// Create a mock that fails twice with rate-limit-like errors, then succeeds
	callCount := 0
	inner := &testRateLimitProvider{
		chatFunc: func(ctx context.Context, messages []core.Message, tools []core.Tool) (*core.AgentOutput, error) {
			callCount++
			if callCount <= 2 {
				return nil, fmt.Errorf("rate limit exceeded (429)")
			}
			return &core.AgentOutput{Content: "success"}, nil
		},
	}

	wrapper := WithRateLimit(inner, 3, 10*time.Millisecond)
	output, err := wrapper.Chat(context.Background(), []core.Message{{Role: "user", Content: "test"}}, nil)
	if err != nil {
		t.Fatalf("expected success after retries, got error: %v", err)
	}
	if output.Content != "success" {
		t.Errorf("expected 'success', got: %s", output.Content)
	}
	if callCount != 3 {
		t.Errorf("expected 3 calls, got: %d", callCount)
	}
}

func TestRateLimitedProviderDoesNotRetryNonRateLimitError(t *testing.T) {
	callCount := 0
	inner := &testRateLimitProvider{
		chatFunc: func(ctx context.Context, messages []core.Message, tools []core.Tool) (*core.AgentOutput, error) {
			callCount++
			return nil, fmt.Errorf("connection refused")
		},
	}

	wrapper := WithRateLimit(inner, 3, 10*time.Millisecond)
	_, err := wrapper.Chat(context.Background(), []core.Message{{Role: "user", Content: "test"}}, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "connection refused") {
		t.Errorf("expected connection refused error, got: %v", err)
	}
	if callCount != 1 {
		t.Errorf("expected 1 call (no retry), got: %d", callCount)
	}
}

func TestRateLimitedProviderImmediateSuccess(t *testing.T) {
	callCount := 0
	inner := &testRateLimitProvider{
		chatFunc: func(ctx context.Context, messages []core.Message, tools []core.Tool) (*core.AgentOutput, error) {
			callCount++
			return &core.AgentOutput{Content: "done"}, nil
		},
	}

	wrapper := WithRateLimit(inner, 3, 10*time.Millisecond)
	output, err := wrapper.Chat(context.Background(), []core.Message{{Role: "user", Content: "test"}}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if output.Content != "done" {
		t.Errorf("expected 'done', got: %s", output.Content)
	}
	if callCount != 1 {
		t.Errorf("expected 1 call, got: %d", callCount)
	}
}

func TestRateLimitedProviderExhaustsRetries(t *testing.T) {
	callCount := 0
	inner := &testRateLimitProvider{
		chatFunc: func(ctx context.Context, messages []core.Message, tools []core.Tool) (*core.AgentOutput, error) {
			callCount++
			return nil, fmt.Errorf("rate limit exceeded")
		},
	}

	wrapper := WithRateLimit(inner, 2, 5*time.Millisecond)
	_, err := wrapper.Chat(context.Background(), []core.Message{{Role: "user", Content: "test"}}, nil)
	if err == nil {
		t.Fatal("expected error after exhausting retries, got nil")
	}
	// Initial attempt + 2 retries = 3 total
	if callCount != 3 {
		t.Errorf("expected 3 calls (1 + maxRetries), got: %d", callCount)
	}
}

func TestIsRateLimitError(t *testing.T) {
	tests := []struct {
		err  string
		want bool
	}{
		{"rate limit exceeded", true},
		{"429 Too Many Requests", true},
		{"throttled by server", true},
		{"too many requests for this endpoint", true},
		{"retry_after: 30", true},
		{"connection refused", false},
		{"auth failed", false},
		{"", false},
	}

	for _, tc := range tests {
		t.Run(tc.err, func(t *testing.T) {
			got := isRateLimitError(fmt.Errorf("%s", tc.err))
			if got != tc.want {
				t.Errorf("isRateLimitError(%q) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}

func TestRateLimitedProviderDelegatesNameAndCapabilities(t *testing.T) {
	inner := &testRateLimitProvider{
		nameFunc: func() string { return "test-provider" },
		capsFunc: func() core.Capabilities { 
			return core.Capabilities{ContextWindow: 8000} 
		},
	}
	wrapper := WithRateLimit(inner, 3, time.Second)

	if !strings.Contains(wrapper.Name(), "test-provider") {
		t.Errorf("name should delegate, got: %s", wrapper.Name())
	}

	caps := wrapper.Capabilities()
	if caps.ContextWindow != 8000 {
		t.Errorf("capabilities should delegate, got context %d, want 8000", caps.ContextWindow)
	}
}

// testRateLimitProvider is a mock provider for testing the rate limiter.
type testRateLimitProvider struct {
	chatFunc func(ctx context.Context, messages []core.Message, tools []core.Tool) (*core.AgentOutput, error)
	nameFunc func() string
	capsFunc func() core.Capabilities
}

func (p *testRateLimitProvider) Chat(ctx context.Context, messages []core.Message, tools []core.Tool) (*core.AgentOutput, error) {
	return p.chatFunc(ctx, messages, tools)
}

func (p *testRateLimitProvider) Name() string {
	if p.nameFunc != nil {
		return p.nameFunc()
	}
	return "test"
}

func (p *testRateLimitProvider) Capabilities() core.Capabilities {
	if p.capsFunc != nil {
		return p.capsFunc()
	}
	return core.Capabilities{}
}
