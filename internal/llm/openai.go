package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"autodev/internal/core"
)

// OpenAIProvider implements the Provider interface for OpenAI-compatible APIs.
type OpenAIProvider struct {
	BaseURL string
	APIKey  string
	Model   string
}

func (p *OpenAIProvider) Name() string {
	return "openai"
}

func (p *OpenAIProvider) Capabilities() core.Capabilities {
	return core.Capabilities{
		MaxTokens:     128000,
		ContextWindow: 128000,
		Streaming:     true,
		Vision:        true,
		FunctionCall:  true,
	}
}

func (p *OpenAIProvider) Chat(ctx context.Context, messages []core.Message, tools []core.Tool) (*core.AgentOutput, error) {
	return p.ChatWithOptions(ctx, messages, tools, ChatOptions{})
}

func (p *OpenAIProvider) ChatWithOptions(ctx context.Context, messages []core.Message, tools []core.Tool, opts ChatOptions) (*core.AgentOutput, error) {
	openaiMessages := make([]map[string]any, len(messages))
	for i, msg := range messages {
		m := map[string]any{
			"role": msg.Role,
		}
		if msg.Content != "" {
			m["content"] = msg.Content
		}
		if msg.Role == "tool" {
			m["tool_call_id"] = msg.ToolCallID
			m["name"] = msg.Name
		}
		openaiMessages[i] = m
	}

	model := p.Model
	if opts.Model != "" {
		model = opts.Model
	}

	reqBody := map[string]any{
		"model":    model,
		"messages": openaiMessages,
	}

	if opts.Temperature > 0 {
		reqBody["temperature"] = opts.Temperature
	}
	if opts.MaxTokens > 0 {
		reqBody["max_tokens"] = opts.MaxTokens
	}
	if len(tools) > 0 {
		reqBody["tools"] = tools
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", p.BaseURL+"/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+p.APIKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error: %s - %s", resp.Status, string(body))
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content   string            `json:"content"`
				ToolCalls []core.ToolCall   `json:"tool_calls"`
			} `json:"message"`
		} `json:"choices"`
		Usage core.Usage `json:"usage"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if result.Error != nil {
		return nil, fmt.Errorf("API Error: %s", result.Error.Message)
	}

	if len(result.Choices) == 0 {
		return nil, fmt.Errorf("empty response")
	}

	choice := result.Choices[0]
	output := &core.AgentOutput{
		Content:   choice.Message.Content,
		ToolCalls: choice.Message.ToolCalls,
		Usage:     result.Usage,
		Model:     model,
	}

	return output, nil
}
