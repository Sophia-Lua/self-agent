package llm

import (
	"context"
	"testing"

	"autodev/internal/core"
)

func TestMockProviderName(t *testing.T) {
	p := &MockProvider{}
	if p.Name() != "mock" {
		t.Errorf("expected 'mock', got '%s'", p.Name())
	}
}

func TestMockProviderCapabilities(t *testing.T) {
	p := &MockProvider{}
	caps := p.Capabilities()

	if caps.MaxTokens != 128000 {
		t.Errorf("MaxTokens = %d, want 128000", caps.MaxTokens)
	}
	if caps.ContextWindow != 128000 {
		t.Errorf("ContextWindow = %d, want 128000", caps.ContextWindow)
	}
	if caps.Streaming {
		t.Error("Streaming should be false for mock")
	}
	if !caps.FunctionCall {
		t.Error("FunctionCall should be true for mock")
	}
}

func TestMockProviderSuccess(t *testing.T) {
	p := &MockProvider{}

	messages := []core.Message{
		{Role: "system", Content: "You are an expert Lead Developer. Analyze the request."},
	}
	output, err := p.Chat(context.Background(), messages, nil)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if output == nil {
		t.Fatal("output is nil")
	}
	if output.Content == "" {
		t.Error("content should not be empty")
	}
}

func TestMockProviderDeveloperRole(t *testing.T) {
	p := &MockProvider{}

	messages := []core.Message{
		{Role: "system", Content: "You are an expert AI Coding Agent. Write clean, efficient code."},
	}
	output, err := p.Chat(context.Background(), messages, []core.Tool{
		{
			Type: "function",
			Function: core.ToolFunction{
				Name:        "write_file",
				Description: "Write a file",
				Parameters:  map[string]any{"type": "object"},
			},
		},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(output.ToolCalls) != 1 {
		t.Errorf("expected 1 tool call, got %d", len(output.ToolCalls))
	}
	if output.ToolCalls[0].Function.Name != "write_file" {
		t.Errorf("expected tool 'write_file', got '%s'", output.ToolCalls[0].Function.Name)
	}
}

func TestMockProviderTesterRole(t *testing.T) {
	p := &MockProvider{}

	messages := []core.Message{
		{Role: "system", Content: "You are an expert QA Engineer. Review the code."},
	}
	output, err := p.Chat(context.Background(), messages, nil)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if output.Content != "[Tester] All tests passed successfully." {
		t.Errorf("unexpected content: %s", output.Content)
	}
}

func TestMockProviderRecoveryRole(t *testing.T) {
	p := &MockProvider{}

	messages := []core.Message{
		{Role: "system", Content: "You are an expert Recovery Agent. Analyze the error."},
	}
	output, err := p.Chat(context.Background(), messages, nil)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if output.Content != "[Recovery] I have fixed the issue by adjusting the prompt." {
		t.Errorf("unexpected content: %s", output.Content)
	}
}

func TestMockProviderFailOnce(t *testing.T) {
	p := &MockProvider{FailCount: 1}

	messages := []core.Message{
		{Role: "system", Content: "You are an expert Lead Developer. Analyze the request."},
	}

	// First call should fail
	_, err := p.Chat(context.Background(), messages, nil)
	if err == nil {
		t.Fatal("first call should fail")
	}

	// Second call should succeed
	output, err := p.Chat(context.Background(), messages, nil)
	if err != nil {
		t.Fatalf("second call should succeed: %v", err)
	}
	if output == nil {
		t.Fatal("output is nil")
	}
}

func TestMockProviderMultipleFailures(t *testing.T) {
	p := &MockProvider{FailCount: 3}

	messages := []core.Message{
		{Role: "system", Content: "You are an expert Lead Developer."},
	}

	for i := 0; i < 3; i++ {
		_, err := p.Chat(context.Background(), messages, nil)
		if err == nil {
			t.Fatalf("call %d should fail", i+1)
		}
	}

	// 4th call should succeed
	output, err := p.Chat(context.Background(), messages, nil)
	if err != nil {
		t.Fatalf("4th call should succeed: %v", err)
	}
	if output == nil {
		t.Fatal("output is nil")
	}
}

func TestMockProviderGenericRole(t *testing.T) {
	p := &MockProvider{}

	messages := []core.Message{
		{Role: "system", Content: "Some unknown system prompt."},
	}
	output, err := p.Chat(context.Background(), messages, nil)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if output.Content != "Ack: Some unknown system prompt." {
		t.Errorf("unexpected content: %s", output.Content)
	}
}

func TestMockProviderWithContext(t *testing.T) {
	p := &MockProvider{}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	messages := []core.Message{
		{Role: "system", Content: "You are a tester."},
	}

	output, err := p.Chat(ctx, messages, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if output.Content == "" {
		t.Error("content should not be empty")
	}
}

func TestFactoryOpenAI(t *testing.T) {
	p, err := NewProvider(ProviderConfig{
		Provider: "openai",
		Model:    "gpt-4",
		APIKey:   "sk-test",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Name() != "openai" {
		t.Errorf("expected 'openai', got '%s'", p.Name())
	}

	// Type check
	if _, ok := p.(*OpenAIProvider); !ok {
		t.Error("expected *OpenAIProvider type")
	}
}

func TestFactoryEmptyProvider(t *testing.T) {
	// Empty provider defaults to openai
	p, err := NewProvider(ProviderConfig{
		Provider: "",
		Model:    "gpt-4",
		APIKey:   "sk-test",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Name() != "openai" {
		t.Errorf("expected 'openai' for empty provider, got '%s'", p.Name())
	}
}

func TestFactoryAzure(t *testing.T) {
	p, err := NewProvider(ProviderConfig{
		Provider: "azure",
		Model:    "gpt-4",
		APIKey:   "azure-key",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Name() != "openai" {
		t.Errorf("azure should map to openai, got '%s'", p.Name())
	}
}

func TestFactoryOpenAICustomBaseURL(t *testing.T) {
	p, err := NewProvider(ProviderConfig{
		Provider: "openai",
		Model:    "gpt-4",
		APIKey:   "sk-test",
		BaseURL:  "https://custom-proxy.example.com/v1",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	openaiP, ok := p.(*OpenAIProvider)
	if !ok {
		t.Fatal("expected *OpenAIProvider")
	}
	if openaiP.BaseURL != "https://custom-proxy.example.com/v1" {
		t.Errorf("BaseURL = '%s', want 'https://custom-proxy.example.com/v1'", openaiP.BaseURL)
	}
}

func TestFactoryOpenAIDefaultBaseURL(t *testing.T) {
	p, err := NewProvider(ProviderConfig{
		Provider: "openai",
		Model:    "gpt-4",
		APIKey:   "sk-test",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	openaiP, ok := p.(*OpenAIProvider)
	if !ok {
		t.Fatal("expected *OpenAIProvider")
	}
	if openaiP.BaseURL != "https://api.openai.com/v1" {
		t.Errorf("default BaseURL = '%s', want 'https://api.openai.com/v1'", openaiP.BaseURL)
	}
}

func TestFactoryClaude(t *testing.T) {
	p, err := NewProvider(ProviderConfig{
		Provider: "claude",
		Model:    "claude-sonnet-4-20250514",
		APIKey:   "sk-ant-test",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Name() != "claude" {
		t.Errorf("expected 'claude', got '%s'", p.Name())
	}

	if _, ok := p.(*ClaudeProvider); !ok {
		t.Error("expected *ClaudeProvider type")
	}
}

func TestFactoryAnthropic(t *testing.T) {
	p, err := NewProvider(ProviderConfig{
		Provider: "anthropic",
		Model:    "claude-sonnet-4",
		APIKey:   "sk-ant-test",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Name() != "claude" {
		t.Errorf("anthropic should map to claude, got '%s'", p.Name())
	}
}

func TestFactoryClaudeCapabilities(t *testing.T) {
	p, err := NewProvider(ProviderConfig{
		Provider: "claude",
		Model:    "claude-3-opus",
		APIKey:   "sk-ant-test",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	caps := p.Capabilities()
	if caps.MaxTokens != 200000 {
		t.Errorf("MaxTokens = %d, want 200000", caps.MaxTokens)
	}
	if caps.ContextWindow != 200000 {
		t.Errorf("ContextWindow = %d, want 200000", caps.ContextWindow)
	}
	if !caps.FunctionCall {
		t.Error("FunctionCall should be true")
	}
}

func TestFactoryOllama(t *testing.T) {
	p, err := NewProvider(ProviderConfig{
		Provider: "ollama",
		Model:    "llama3",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Name() != "ollama" {
		t.Errorf("expected 'ollama', got '%s'", p.Name())
	}

	if _, ok := p.(*OllamaProvider); !ok {
		t.Error("expected *OllamaProvider type")
	}
}

func TestFactoryLocal(t *testing.T) {
	p, err := NewProvider(ProviderConfig{
		Provider: "local",
		Model:    "mistral",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Name() != "ollama" {
		t.Errorf("local should map to ollama, got '%s'", p.Name())
	}
}

func TestFactoryOllamaDefaultBaseURL(t *testing.T) {
	p, err := NewProvider(ProviderConfig{
		Provider: "ollama",
		Model:    "llama3",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ollamaP, ok := p.(*OllamaProvider)
	if !ok {
		t.Fatal("expected *OllamaProvider")
	}
	if ollamaP.BaseURL != "http://localhost:11434" {
		t.Errorf("default BaseURL = '%s', want 'http://localhost:11434'", ollamaP.BaseURL)
	}
}

func TestFactoryOllamaCustomBaseURL(t *testing.T) {
	p, err := NewProvider(ProviderConfig{
		Provider: "ollama",
		Model:    "llama3",
		BaseURL:  "http://custom-server:11434",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ollamaP, ok := p.(*OllamaProvider)
	if !ok {
		t.Fatal("expected *OllamaProvider")
	}
	if ollamaP.BaseURL != "http://custom-server:11434" {
		t.Errorf("BaseURL = '%s', want 'http://custom-server:11434'", ollamaP.BaseURL)
	}
}

func TestFactoryOllamaCapabilities(t *testing.T) {
	p, err := NewProvider(ProviderConfig{
		Provider: "ollama",
		Model:    "llama3",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	caps := p.Capabilities()
	if caps.MaxTokens != 8192 {
		t.Errorf("MaxTokens = %d, want 8192", caps.MaxTokens)
	}
	if caps.Vision {
		t.Error("Vision should be false for ollama")
	}
	if caps.FunctionCall {
		t.Error("FunctionCall should be false for most ollama models")
	}
}

func TestFactoryMock(t *testing.T) {
	p, err := NewProvider(ProviderConfig{
		Provider: "mock",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Name() != "mock" {
		t.Errorf("expected 'mock', got '%s'", p.Name())
	}

	if _, ok := p.(*MockProvider); !ok {
		t.Error("expected *MockProvider type")
	}
}

func TestFactoryUnsupported(t *testing.T) {
	_, err := NewProvider(ProviderConfig{
		Provider: "unknown",
	})

	if err == nil {
		t.Error("expected error for unsupported provider")
	}
}

func TestNewClaudeProvider(t *testing.T) {
	p := NewClaudeProvider("sk-ant-key", "claude-3-5-sonnet")

	if p.Name() != "claude" {
		t.Errorf("expected 'claude', got '%s'", p.Name())
	}
	if p.APIKey != "sk-ant-key" {
		t.Errorf("APIKey = '%s', want 'sk-ant-key'", p.APIKey)
	}
	if p.Model != "claude-3-5-sonnet" {
		t.Errorf("Model = '%s', want 'claude-3-5-sonnet'", p.Model)
	}
	if p.BaseURL != "https://api.anthropic.com" {
		t.Errorf("BaseURL = '%s', want 'https://api.anthropic.com'", p.BaseURL)
	}
}

func TestNewOllamaProvider(t *testing.T) {
	p := NewOllamaProvider("http://myserver:11434", "mistral")

	if p.Name() != "ollama" {
		t.Errorf("expected 'ollama', got '%s'", p.Name())
	}
	if p.Model != "mistral" {
		t.Errorf("Model = '%s', want 'mistral'", p.Model)
	}
	if p.BaseURL != "http://myserver:11434" {
		t.Errorf("BaseURL = '%s', want 'http://myserver:11434'", p.BaseURL)
	}
}

func TestOpenAIProviderName(t *testing.T) {
	p := &OpenAIProvider{APIKey: "sk-test", Model: "gpt-4"}
	if p.Name() != "openai" {
		t.Errorf("Name() = '%s', want 'openai'", p.Name())
	}
}

func TestOpenAICapabilities(t *testing.T) {
	p := &OpenAIProvider{}
	caps := p.Capabilities()

	if caps.MaxTokens != 128000 {
		t.Errorf("MaxTokens = %d, want 128000", caps.MaxTokens)
	}
	if !caps.Streaming {
		t.Error("Streaming should be true")
	}
	if !caps.Vision {
		t.Error("Vision should be true")
	}
	if !caps.FunctionCall {
		t.Error("FunctionCall should be true")
	}
}
