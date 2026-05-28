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

// ClaudeProvider implements the Provider interface for Anthropic's Claude API.
type ClaudeProvider struct {
	BaseURL string
	APIKey  string
	Model   string
}

func NewClaudeProvider(apiKey, model string) *ClaudeProvider {
	return &ClaudeProvider{
		BaseURL: "https://api.anthropic.com",
		APIKey:  apiKey,
		Model:   model,
	}
}

func (p *ClaudeProvider) Name() string {
	return "claude"
}

func (p *ClaudeProvider) Capabilities() core.Capabilities {
	return core.Capabilities{
		MaxTokens:     200000,
		ContextWindow: 200000,
		Streaming:     true,
		Vision:        true,
		FunctionCall:  true,
	}
}

func (p *ClaudeProvider) Chat(ctx context.Context, messages []core.Message, tools []core.Tool) (*core.AgentOutput, error) {
	return p.ChatWithOptions(ctx, messages, tools, ChatOptions{})
}

func (p *ClaudeProvider) ChatWithOptions(ctx context.Context, messages []core.Message, tools []core.Tool, opts ChatOptions) (*core.AgentOutput, error) {
	systemPrompt := ""
	nonSystemMessages := make([]core.Message, 0, len(messages))
	for _, msg := range messages {
		if msg.Role == "system" {
			systemPrompt = msg.Content
		} else {
			nonSystemMessages = append(nonSystemMessages, msg)
		}
	}

	model := p.Model
	if opts.Model != "" {
		model = opts.Model
	}

	reqBody := map[string]any{
		"model":      model,
		"messages":   nonSystemMessages,
		"max_tokens": 4096,
	}

	if opts.Temperature > 0 {
		reqBody["temperature"] = opts.Temperature
	}
	if opts.MaxTokens > 0 {
		reqBody["max_tokens"] = opts.MaxTokens
	}
	if systemPrompt != "" {
		reqBody["system"] = systemPrompt
	}

	if len(tools) > 0 {
		claudeTools := make([]map[string]any, len(tools))
		for i, t := range tools {
			claudeTools[i] = map[string]any{
				"name":        t.Function.Name,
				"description": t.Function.Description,
				"input_schema": t.Function.Parameters,
			}
		}
		reqBody["tools"] = claudeTools
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", p.BaseURL+"/v1/messages", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}

	req.Header.Set("x-api-key", p.APIKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("content-type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Claude API error: %s - %s", resp.Status, string(body))
	}

	var result struct {
		ID         string `json:"id"`
		Type       string `json:"type"`
		Role       string `json:"role"`
		Model      string `json:"model"`
		Content    []struct {
			Type  string `json:"type"`
			Text  string `json:"text,omitempty"`
			ID    string `json:"id,omitempty"`
			Name  string `json:"name,omitempty"`
			Input any    `json:"input,omitempty"`
		} `json:"content"`
		StopReason string   `json:"stop_reason"`
		StopSeq    *string  `json:"stop_sequence"`
		Usage      struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	output := &core.AgentOutput{
		Model: result.Model,
		Usage: core.Usage{
			PromptTokens:     result.Usage.InputTokens,
			CompletionTokens: result.Usage.OutputTokens,
			TotalTokens:      result.Usage.InputTokens + result.Usage.OutputTokens,
		},
	}

	for _, block := range result.Content {
		switch block.Type {
		case "text":
			output.Content += block.Text
		case "tool_use":
			argsBytes, _ := json.Marshal(block.Input)
			output.ToolCalls = append(output.ToolCalls, core.ToolCall{
				ID:   block.ID,
				Type: "function",
				Function: core.FunctionCall{
					Name:      block.Name,
					Arguments: string(argsBytes),
				},
			})
		}
	}

	return output, nil
}
