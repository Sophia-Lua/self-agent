package context

import (
	"fmt"
	"strings"

	"autodev/internal/core"
)

// Builder constructs LLM-friendly context from raw files and tasks.
// It applies token estimation and truncation strategies to fit the context within limits.
type Builder struct {
	// MaxTokens is the hard limit for the total context window.
	// If the context exceeds this, files will be truncated or removed.
	MaxTokens int
	
	// TokenEstimator is a function to estimate token count.
	// Default is a simple character count / 4.
	TokenEstimator func(text string) int
}

// NewBuilder creates a new ContextBuilder with defaults.
func NewBuilder() *Builder {
	return &Builder{
		MaxTokens:    128000, // Default safe limit for GPT-4o
		TokenEstimator: EstimateTokensSimple,
	}
}

// Build assembles the input for the agent.
func (b *Builder) Build(task string, systemPrompt string, history []core.Message, files map[string]string) ([]core.Message, error) {
	// Default Token Estimator if not provided
	estimator := b.TokenEstimator
	if estimator == nil {
		estimator = EstimateTokensSimple
	}

	messages := make([]core.Message, 0)

	// 1. System Prompt
	messages = append(messages, core.Message{
		Role:    "system",
		Content: systemPrompt,
	})

	// 2. History
	for _, msg := range history {
		messages = append(messages, msg)
	}

	// 3. Estimate used tokens
	usedTokens := 0
	for _, m := range messages {
		usedTokens += estimator(m.Content)
	}

	// Reserve space for the task and a minimal response structure
	reserved := 1024 // Task text + buffer
	if usedTokens+reserved >= b.MaxTokens {
		return nil, fmt.Errorf("history already exceeds context limit (%d tokens used)", usedTokens)
	}

	availableForFiles := b.MaxTokens - usedTokens - reserved

	// 4. Construct file context
	var ctxContent strings.Builder
	ctxContent.WriteString("Here is the current project file structure. Please refer to these files to complete the task.\n")
	ctxContent.WriteString("If a file content is truncated, assume it continues based on standard coding practices.\n")
	ctxContent.WriteString("---\n")

	filesIncluded := 0
	for path, content := range files {
		fileTokens := estimator(content)
		
		if availableForFiles <= 0 {
			break // Budget exhausted
		}

		ctxContent.WriteString("**File: " + path + "**\n")
		
		if fileTokens <= availableForFiles {
			// Fit whole
			ctxContent.WriteString("```"+path+"\n" + content + "\n```\n")
			availableForFiles -= fileTokens
			filesIncluded++
		} else {
			// Truncate
			// We need to leave some chars for the "..." and closing ```
			// Simple truncation: take first X chars that fit tokens
			approxChars := availableForFiles * 4
			if approxChars > len(content) {
				approxChars = len(content)
			} else {
				approxChars -= 50 // Safety padding
			}
			
			// Sanity check: if not enough space to truncate safely, omit file
			if approxChars < 10 {
				ctxContent.WriteString("*[File Omitted due to context limit]*\n")
			} else {
				ctxContent.WriteString("```"+path+"\n" + content[:approxChars] + "\n...\n```\n")
				ctxContent.WriteString("*[File Truncated due to length]*\n")
				availableForFiles = 0
			}
		}
		ctxContent.WriteString("---\n")
	}

	// 5. Append Task
	userContent := fmt.Sprintf("\n### Task Description ###\n%s\n\n### Response Guidelines ###\n1. Analyze the provided files.\n2. If you need to modify files, use the `write_file` tool.\n3. If you need to explain, just write text.", task)
	
	if estimator(userContent)+usedTokens > b.MaxTokens {
		// Task is too long? Unlikely, but technically possible.
		return nil, fmt.Errorf("task description exceeds remaining context")
	}

	messages = append(messages, core.Message{
		Role:    "user",
		Content: ctxContent.String() + userContent,
	})

	return messages, nil
}

// EstimateTokensSimple uses a 4:1 character-to-token ratio.
// This is a rough heuristic suitable for rough bounding, not billing.
func EstimateTokensSimple(text string) int {
	if text == "" {
		return 0
	}
	return len(text) / 4
}
