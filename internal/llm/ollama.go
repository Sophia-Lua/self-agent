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

// OllamaProvider implements the Provider interface for local Ollama.
type OllamaProvider struct {
	BaseURL string
	Model   string
}

func NewOllamaProvider(baseURL, model string) *OllamaProvider {
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}
	return &OllamaProvider{
		BaseURL: baseURL,
		Model:   model,
	}
}

func (p *OllamaProvider) Name() string {
	return "ollama"
}

func (p *OllamaProvider) Capabilities() core.Capabilities {
	// Ollama model capabilities vary by model; provide reasonable defaults.
	return core.Capabilities{
		MaxTokens:     8192,
		ContextWindow: 8192,
		Streaming:     true,
		Vision:        false,
		FunctionCall:  false, // Most Ollama models don't support tool calls natively
	}
}

func (p *OllamaProvider) Chat(ctx context.Context, messages []core.Message, tools []core.Tool) (*core.AgentOutput, error) {
	return p.ChatWithOptions(ctx, messages, tools, ChatOptions{})
}

func (p *OllamaProvider) ChatWithOptions(ctx context.Context, messages []core.Message, tools []core.Tool, opts ChatOptions) (*core.AgentOutput, error) {
	model := p.Model
	if opts.Model != "" {
		model = opts.Model
	}

	reqBody := map[string]any{
		"model":   model,
		"messages": messages,
		"stream":  false,
	}

	if opts.Temperature > 0 {
		reqBody["temperature"] = opts.Temperature
	}
	if opts.MaxTokens > 0 {
		reqBody["options"] = map[string]any{"num_predict": opts.MaxTokens}
	}
	if len(tools) > 0 {
		reqBody["tools"] = tools
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", p.BaseURL+"/api/chat", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Ollama API error: %s - %s", resp.Status, string(body))
	}

	var result struct {
		Model      string `json:"model"`
		CreatedAt  string `json:"created_at"`
		Message    struct {
			Role      string     `json:"role"`
			Content   string     `json:"content"`
			ToolCalls []struct {
				Function struct {
					Name      string         `json:"name"`
					Arguments map[string]any `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"message"`
		Done    bool `json:"done"`
		Usage   struct {
			PromptTokens     int `json:"prompt_eval_count"`
			CompletionTokens int `json:"eval_count"`
			TotalDuration    int `json:"total_duration"`
		} `json:"usage,omitempty"`
		PromptEvalCount int `json:"prompt_eval_count"`
		EvalCount       int `json:"eval_count"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	output := &core.AgentOutput{
		Model:   result.Model,
		Content: result.Message.Content,
		Usage: core.Usage{
			PromptTokens:     result.PromptEvalCount,
			CompletionTokens: result.EvalCount,
			TotalTokens:      result.PromptEvalCount + result.EvalCount,
		},
	}

	for _, tc := range result.Message.ToolCalls {
		argsBytes, _ := json.Marshal(tc.Function.Arguments)
		output.ToolCalls = append(output.ToolCalls, core.ToolCall{
			ID:   "",
			Type: "function",
			Function: core.FunctionCall{
				Name:      tc.Function.Name,
				Arguments: string(argsBytes),
			},
		})
	}

	return output, nil
}
