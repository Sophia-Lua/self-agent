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

func (p *MockProvider) Chat(ctx context.Context, messages []core.Message) (string, error) {
	p.callCount++
	
	// Simulate different responses based on the System Prompt
	systemMsg := messages[0]
	
	// Trigger failure on the first call if configured
	if p.callCount <= p.FailCount {
		if strings.Contains(systemMsg.Content, "Lead Developer") || strings.Contains(systemMsg.Content, "Coding Agent") {
			return "", fmt.Errorf("mock llm error: rate limit exceeded or timeout")
		}
	}
	
	var response string
	if strings.Contains(systemMsg.Content, "Lead Developer") {
		response = "[Parser] I will analyze the request and break it down."
	} else if strings.Contains(systemMsg.Content, "Coding Agent") {
		response = "[Developer] I have generated the necessary code: `print('hello world')`"
	} else if strings.Contains(systemMsg.Content, "QA Engineer") {
		response = "[Tester] All tests passed successfully."
	} else if strings.Contains(systemMsg.Content, "Recovery Agent") {
		response = "[Recovery] I have fixed the issue by adjusting the prompt."
	} else {
		response = fmt.Sprintf("Ack: %s", systemMsg.Content)
	}

	return response, nil
}
