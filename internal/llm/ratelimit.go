package llm

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"autodev/internal/core"
)

// RetryClient wraps an HTTP client with retry logic for rate-limited requests.
type RetryClient struct {
	client    *http.Client
	maxRetries int
	baseDelay  time.Duration
}

// NewRetryClient creates a client with rate-limit-aware retry logic.
func NewRetryClient(maxRetries int, baseDelay time.Duration) *RetryClient {
	if maxRetries <= 0 {
		maxRetries = 3
	}
	if baseDelay <= 0 {
		baseDelay = 1 * time.Second
	}
	return &RetryClient{
		client:     &http.Client{Timeout: 120 * time.Second},
		maxRetries: maxRetries,
		baseDelay:  baseDelay,
	}
}

// Do executes an HTTP request with automatic retry on rate limit errors.
func (c *RetryClient) Do(req *http.Request) (*http.Response, error) {
	var lastErr error

	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		if attempt > 0 {
			// Check for Retry-After header in case of previous response
			// This is handled by the caller passing the response
			delay := c.calculateDelay(attempt, 0)
			if delay > 0 {
				select {
				case <-time.After(delay):
				case <-req.Context().Done():
					return nil, req.Context().Err()
				}
			}
		}

		resp, err := c.client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}

		if resp.StatusCode != http.StatusTooManyRequests {
			return resp, nil
		}

		resp.Body.Close()
		lastErr = &RateLimitError{StatusCode: resp.StatusCode, RetryAfter: resp.Header.Get("Retry-After")}

		// Parse Retry-After header
		retryAfter := parseRetryAfter(resp.Header.Get("Retry-After"))
		delay := c.calculateDelay(attempt, retryAfter)

		select {
		case <-time.After(delay):
		case <-req.Context().Done():
			return nil, req.Context().Err()
		}
	}

	return nil, lastErr
}

func (c *RetryClient) calculateDelay(attempt int, retryAfter time.Duration) time.Duration {
	if retryAfter > 0 {
		return retryAfter + jitter(200*time.Millisecond)
	}

	delay := c.baseDelay * time.Duration(math.Pow(2, float64(attempt-1)))
	delay += jitter(500 * time.Millisecond)

	// Cap max delay at 60 seconds
	if delay > 60*time.Second {
		delay = 60*time.Second + jitter(500*time.Millisecond)
	}

	return delay
}

func parseRetryAfter(value string) time.Duration {
	if value == "" {
		return 0
	}

	// Try parsing as integer seconds
	var secs int
	if _, err := fmt.Sscanf(value, "%d", &secs); err == nil && secs > 0 {
		return time.Duration(secs) * time.Second
	}

	// Try parsing as HTTP date
	t, err := time.Parse(time.RFC1123, value)
	if err == nil {
		return time.Until(t)
	}

	// Try as direct duration
	d, err := time.ParseDuration(value)
	if err == nil {
		return d
	}

	return 0
}

func jitter(max time.Duration) time.Duration {
	return time.Duration(rand.Int63n(int64(max)))
}

// RateLimitError indicates the request was rate-limited.
type RateLimitError struct {
	StatusCode int
	RetryAfter string
}

func (e *RateLimitError) Error() string {
	return "rate limit exceeded"
}

// RateLimitedProvider wraps any Provider and adds rate-limit-aware retry logic to all Chat calls.
type RateLimitedProvider struct {
	inner      Provider
	maxRetries int
	baseDelay  time.Duration
}

// WithRateLimit wraps a provider with rate limiting and retry logic.
func WithRateLimit(inner Provider, maxRetries int, baseDelay time.Duration) *RateLimitedProvider {
	return &RateLimitedProvider{
		inner:      inner,
		maxRetries: maxRetries,
		baseDelay:  baseDelay,
	}
}

func (p *RateLimitedProvider) Name() string {
	return p.inner.Name() + " (rate-limited)"
}

func (p *RateLimitedProvider) Chat(ctx context.Context, messages []core.Message, tools []core.Tool) (*core.AgentOutput, error) {
	return p.ChatWithOptions(ctx, messages, tools, ChatOptions{})
}

func (p *RateLimitedProvider) ChatWithOptions(ctx context.Context, messages []core.Message, tools []core.Tool, opts ChatOptions) (*core.AgentOutput, error) {
	var lastErr error

	for attempt := 0; attempt <= p.maxRetries; attempt++ {
		if attempt > 0 {
			delay := p.calculateDelay(attempt)
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}

		var output *core.AgentOutput
		var err error
		if optsInner, ok := p.inner.(ChatWithOpts); ok {
			output, err = optsInner.ChatWithOptions(ctx, messages, tools, opts)
		} else {
			output, err = p.inner.Chat(ctx, messages, tools)
		}
		if err == nil {
			return output, nil
		}

		lastErr = err

		// Only retry on rate-limit-like errors
		if !isRateLimitError(err) {
			return nil, err
		}
	}

	return nil, lastErr
}

func (p *RateLimitedProvider) Capabilities() core.Capabilities {
	return p.inner.Capabilities()
}

func (p *RateLimitedProvider) calculateDelay(attempt int) time.Duration {
	delay := p.baseDelay * time.Duration(math.Pow(2, float64(attempt-1)))
	delay += time.Duration(rand.Int63n(int64(p.baseDelay / 2)))

	if delay > 60*time.Second {
		delay = 60*time.Second + time.Duration(rand.Int63n(int64(1*time.Second)))
	}

	return delay
}

// isRateLimitError checks if an error is related to rate limiting.
func isRateLimitError(err error) bool {
	if err == nil {
		return false
	}

	msg := err.Error()
	lower := strings.ToLower(msg)

	if strings.Contains(lower, "429") ||
		strings.Contains(lower, "rate limit") ||
		strings.Contains(lower, "too many requests") ||
		strings.Contains(lower, "throttl") ||
		strings.Contains(lower, "retry_after") {
		return true
	}

	// Check for wrapped RateLimitError
	var rateErr *RateLimitError
	if err == rateErr {
		return true
	}

	return false
}
