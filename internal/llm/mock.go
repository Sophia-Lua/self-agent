package llm

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"

	"autodev/internal/core"
)

// MockProvider is used for testing the pipeline without hitting real APIs.
type MockProvider struct {
	FailCount int
	callCount atomic.Int64
}

func (p *MockProvider) Name() string {
	return "mock"
}

func (p *MockProvider) Capabilities() core.Capabilities {
	return core.Capabilities{
		MaxTokens:     128000,
		ContextWindow: 128000,
		Streaming:     false,
		Vision:        false,
		FunctionCall:  true,
	}
}

func (p *MockProvider) Chat(ctx context.Context, messages []core.Message, tools []core.Tool) (*core.AgentOutput, error) {
	return p.ChatWithOptions(ctx, messages, tools, ChatOptions{})
}

func (p *MockProvider) ChatWithOptions(ctx context.Context, messages []core.Message, tools []core.Tool, opts ChatOptions) (*core.AgentOutput, error) {
	p.callCount.Add(1)

	if len(messages) == 0 {
		return nil, fmt.Errorf("mock llm error: messages slice is empty")
	}

	systemMsg := messages[0]

	if p.callCount.Load() <= int64(p.FailCount) {
		if strings.Contains(systemMsg.Content, "Lead Developer") || strings.Contains(systemMsg.Content, "Coding Agent") {
			return nil, fmt.Errorf("mock llm error: rate limit exceeded or timeout")
		}
	}

	model := "mock-model"
	if opts.Model != "" {
		model = opts.Model
	}

	if len(tools) > 0 && strings.Contains(systemMsg.Content, "Coding Agent") {
		return &core.AgentOutput{
			Model: model,
			Content: "I will use the tool to write the code.",
			ToolCalls: []core.ToolCall{
				{
					ID:   "call_123",
					Type: "function",
					Function: core.FunctionCall{
						Name:      "write_file",
						Arguments: `{"path": "main.py", "content": "print('Hello Snake Game')"}`,
					},
				},
			},
		}, nil
	}
	
	var content string
	if strings.Contains(systemMsg.Content, "Lead Developer") {
		content = "[Parser] I will analyze the request and break it down."
	} else if strings.Contains(systemMsg.Content, "Coding Agent") {
		content = "[Developer] I have generated the necessary code: `print('hello world')`"
	} else if strings.Contains(systemMsg.Content, "QA Engineer") {
		content = "[Tester] All tests passed successfully."
	} else if strings.Contains(systemMsg.Content, "Recovery Agent") {
		content = "[Recovery] I have fixed the issue by adjusting the prompt."
	} else {
		content = fmt.Sprintf("Ack: %s", systemMsg.Content)
	}

	return &core.AgentOutput{
		Model:   model,
		Content: content,
	}, nil
}
