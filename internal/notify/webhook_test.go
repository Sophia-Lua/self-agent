package notify

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"autodev/internal/events"
)

func TestWebhookSender_SendSuccess(t *testing.T) {
	var received []byte
	var mu sync.Mutex
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		buf := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(buf)
		received = buf
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	sender := NewWebhookSender(SenderConfig{
		URLs:    []string{server.URL},
		Timeout: "5s",
		Retries: 0,
	})

	handler := sender.Handler()
	err := handler(context.Background(), events.Event{
		Type:  events.TypeAgentStart,
		Agent: "agent-parser",
		Payload: map[string]interface{}{"task": "parse task"},
	})
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}

	if len(received) == 0 {
		t.Fatal("received empty payload")
	}

	mu.Lock()
	payloadStr := string(received)
	mu.Unlock()
	if payloadStr == "" {
		t.Fatal("payload should not be empty")
	}
}

func TestWebhookSender_SendFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	sender := NewWebhookSender(SenderConfig{
		URLs:    []string{server.URL},
		Timeout: "1s",
		Retries: 1,
	})

	handler := sender.Handler()
	err := handler(context.Background(), events.Event{
		Type: events.TypeAgentError,
		Payload: map[string]interface{}{"error": "test error"},
	})
	if err == nil {
		t.Fatal("expected error for failing server")
	}
}

func TestWebhookSender_ContextTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second)
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	sender := NewWebhookSender(SenderConfig{
		URLs:    []string{server.URL},
		Timeout: "10s",
		Retries: 0,
	})

	handler := sender.Handler()
	err := handler(ctx, events.Event{Type: events.TypeStateChange})
	if err == nil {
		t.Fatal("expected context timeout error")
	}
}

func TestWebhookSender_SubscribeAll(t *testing.T) {
	var count int
	var mu sync.Mutex
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		count++
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	sender := NewWebhookSender(SenderConfig{
		URLs: []string{server.URL},
	})

	bus := events.NewInMemoryBus()
	if err := sender.SubscribeAll(bus); err != nil {
		t.Fatalf("SubscribeAll failed: %v", err)
	}

	eventsToPublish := []events.Type{
		events.TypeStateChange,
		events.TypeAgentStart,
		events.TypeAgentComplete,
		events.TypeAgentError,
		events.TypePipelineStart,
		events.TypePipelineEnd,
	}
	for _, et := range eventsToPublish {
		_ = bus.Publish(context.Background(), events.Event{Type: et})
	}

	mu.Lock()
	got := count
	mu.Unlock()
	if got != len(eventsToPublish) {
		t.Errorf("expected %d webhook calls, got %d", len(eventsToPublish), got)
	}
}

func TestWebhookSender_SecretHeader(t *testing.T) {
	var gotSecret string
	var mu sync.Mutex
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		gotSecret = r.Header.Get("X-Webhook-Signature")
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	sender := NewWebhookSender(SenderConfig{
		URLs:   []string{server.URL},
		Secret: "my-secret-token",
	})

	handler := sender.Handler()
	_ = handler(context.Background(), events.Event{Type: events.TypeStateChange})

	mu.Lock()
	if gotSecret != "my-secret-token" {
		t.Errorf("expected secret header %q, got %q", "my-secret-token", gotSecret)
	}
	mu.Unlock()
}

func TestWebhookSender_MultipleURLs(t *testing.T) {
	var count1, count2 int
	var mu sync.Mutex

	s1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		count1++
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer s1.Close()

	s2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		count2++
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer s2.Close()

	sender := NewWebhookSender(SenderConfig{
		URLs:    []string{s1.URL, s2.URL},
		Retries: 0,
	})

	handler := sender.Handler()
	_ = handler(context.Background(), events.Event{Type: events.TypeAgentComplete})

	mu.Lock()
	if count1 != 1 || count2 != 1 {
		t.Errorf("expected each URL called once, got s1=%d s2=%d", count1, count2)
	}
	mu.Unlock()
}
