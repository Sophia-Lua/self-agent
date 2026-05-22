package llm

import (
	"context"
	"fmt"
	"strings"

	"autodev/internal/core"
)

// MockProvider is used for testing the pipeline without hitting real APIs.
type MockProvider struct {
	FailCount int // Number of times to fail before succeeding
	callCount int
}

func (p *MockProvider) Name() string {
	return "mock"
}

func (p *MockProvider) Chat(ctx context.Context, messages []core.Message, tools []core.Tool) (*core.AgentOutput, error) {
	p.callCount++
	
	// Simulate different responses based on the System Prompt
	systemMsg := messages[0]
	
	// Trigger failure on the first call if configured
	if p.callCount <= p.FailCount {
		if strings.Contains(systemMsg.Content, "Lead Developer") || strings.Contains(systemMsg.Content, "Coding Agent") {
			return nil, fmt.Errorf("mock llm error: rate limit exceeded or timeout")
		}
	}

	// For Mock Provider, we simulate returning a tool call if tools are present
	if len(tools) > 0 && strings.Contains(systemMsg.Content, "Coding Agent") {
		return &core.AgentOutput{
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
		Content: content,
	}, nil
}
