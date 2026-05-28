package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"autodev/internal/events"
)

// WebhookSender sends notifications to configured webhook URLs.
type WebhookSender struct {
	urls    []string
	client  *http.Client
	secret  string
	retries int
}

// WebhookPayload represents the JSON structure sent to webhook endpoints.
type WebhookPayload struct {
	Type      string                 `json:"type"`
	Agent     string                 `json:"agent,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
	Payload   map[string]interface{} `json:"payload"`
}

// SenderConfig defines webhook sender settings.
type SenderConfig struct {
	URLs    []string `yaml:"urls"`
	Secret  string   `yaml:"secret"`
	Timeout string   `yaml:"timeout"`
	Retries int      `yaml:"retries"`
}

// NewWebhookSender creates a webhook sender from configuration.
func NewWebhookSender(cfg SenderConfig) *WebhookSender {
	timeout := 10 * time.Second
	if cfg.Timeout != "" {
		if d, err := time.ParseDuration(cfg.Timeout); err == nil {
			timeout = d
		}
	}

	retries := cfg.Retries
	if retries <= 0 {
		retries = 2
	}

	return &WebhookSender{
		urls: cfg.URLs,
		client: &http.Client{
			Timeout: timeout,
		},
		secret:  cfg.Secret,
		retries: retries,
	}
}

// Handler returns an events.Handler that sends notifications to all configured URLs.
func (s *WebhookSender) Handler() events.Handler {
	return func(ctx context.Context, event events.Event) error {
		payload := WebhookPayload{
			Type:      string(event.Type),
			Agent:     event.Agent,
			Timestamp: time.Now(),
			Payload:   event.Payload,
		}

		body, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("marshal webhook payload: %w", err)
		}

		var lastErr error
		for _, url := range s.urls {
			if err := s.sendWithRetry(ctx, url, body); err != nil {
				lastErr = fmt.Errorf("webhook %s: %w", url, err)
			}
		}
		return lastErr
	}
}

func (s *WebhookSender) sendWithRetry(ctx context.Context, url string, body []byte) error {
	var lastErr error
	for attempt := 0; attempt <= s.retries; attempt++ {
		if attempt > 0 {
			delay := time.Duration(attempt) * 2 * time.Second
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
		if err != nil {
			return err
		}

		req.Header.Set("Content-Type", "application/json")
		if s.secret != "" {
			req.Header.Set("X-Webhook-Signature", s.secret)
		}

		resp, err := s.client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		resp.Body.Close()

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return nil
		}
		lastErr = fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return lastErr
}

// SubscribeAll registers the webhook handler for all event types on the bus.
func (s *WebhookSender) SubscribeAll(bus events.Bus) error {
	h := s.Handler()
	eventTypes := []events.Type{
		events.TypeStateChange,
		events.TypeAgentStart,
		events.TypeAgentComplete,
		events.TypeAgentError,
		events.TypePipelineStart,
		events.TypePipelineEnd,
	}
	for _, et := range eventTypes {
		if err := bus.Subscribe(et, h); err != nil {
			return fmt.Errorf("subscribe %s: %w", et, err)
		}
	}
	return nil
}
