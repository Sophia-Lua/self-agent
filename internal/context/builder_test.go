package context_test

import (
	"autodev/internal/context"
	"autodev/internal/core"
	"testing"
)

func TestNewBuilder(t *testing.T) {
	b := context.NewBuilder()
	if b == nil {
		t.Fatal("expected ContextBuilder, got nil")
	}
	if b.MaxTokens != 128000 {
		t.Fatalf("expected MaxTokens 128000, got %d", b.MaxTokens)
	}
	if b.TokenEstimator == nil {
		t.Fatal("expected default TokenEstimator")
	}
}

func TestEstimateTokensSimple(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"", 0},
		{"hello", 1},
		{"hello world", 2},
		{"1234", 1},
		{"12345", 1},
	}

	for _, tc := range tests {
		got := context.EstimateTokensSimple(tc.input)
		if got != tc.want {
			t.Errorf("EstimateTokensSimple(%q) = %d, want %d", tc.input, got, tc.want)
		}
	}
}

func TestBuilderBuildBasic(t *testing.T) {
	b := context.NewBuilder()
	b.MaxTokens = 32000

	messages, err := b.Build(
		"Create a hello world script",
		"You are a helpful assistant.",
		[]core.Message{
			{Role: "user", Content: "Start a new project"},
			{Role: "assistant", Content: "OK, I'll help you."},
		},
		map[string]string{
			"main.go": `package main

import "fmt"

func main() {
	fmt.Println("hello world")
}`,
			"README.md": "# Project",
		},
	)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	if len(messages) == 0 {
		t.Fatal("expected messages, got none")
	}

	if messages[0].Role != "system" {
		t.Errorf("expected first message role 'system', got '%s'", messages[0].Role)
	}

	if messages[len(messages)-1].Role != "user" {
		t.Errorf("expected last message role 'user', got '%s'", messages[len(messages)-1].Role)
	}
}

func TestBuilderContextExceeds(t *testing.T) {
	b := context.NewBuilder()
	b.MaxTokens = 50

	longPrompt := "You are a very verbose assistant. " + string(make([]byte, 1000))
	_, err := b.Build(
		"Task",
		longPrompt,
		nil,
		nil,
	)
	if err == nil {
		t.Fatal("expected error when context exceeds limit")
	}
}

func TestBuildWithNoFiles(t *testing.T) {
	b := context.NewBuilder()
	b.MaxTokens = 32000

	messages, err := b.Build(
		"Create something",
		"You are helpful.",
		nil,
		nil,
	)
	if err != nil {
		t.Fatalf("Build with no files should succeed: %v", err)
	}
	if len(messages) != 2 {
		t.Fatalf("expected 2 messages (system + user), got %d", len(messages))
	}
}
