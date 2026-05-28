package pipeline_test

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"autodev/internal/agents"
	"autodev/internal/core"
	ctxbuilder2 "autodev/internal/context"
	"autodev/internal/events"
	"autodev/internal/llm"
	"autodev/internal/memory"
	"autodev/internal/pipeline"
	"autodev/internal/tools"
)

func TestPipelineHappyPath(t *testing.T) {
	dir := t.TempDir()

	testFile := filepath.Join(dir, "hello.txt")
	if err := os.WriteFile(testFile, []byte("initial content"), 0644); err != nil {
		t.Fatal(err)
	}

	prov := &llm.MockProvider{}
	ctxBuilder := &ctxbuilder2.Builder{MaxTokens: 32000}
	toolReg := tools.New()
	tools.RegisterFileTools(toolReg, dir)

	mem, err := memory.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}

	bus := events.NewInMemoryBus()
	cfg := &core.Config{WorkDir: dir}

	reg := agents.NewRegistry()
	reg.Register(&agents.Executor{
		AgentID:   "agent-parser",
		AgentRole: core.RoleParser,
		Provider:  prov,
		Context:   ctxBuilder,
	})
	reg.Register(&agents.Executor{
		AgentID:      "agent-developer",
		AgentRole:    core.RoleDeveloper,
		Provider:     prov,
		Context:      ctxBuilder,
		ToolRegistry: toolReg,
	})
	reg.Register(&agents.Executor{
		AgentID:   "agent-tester",
		AgentRole: core.RoleTester,
		Provider:  prov,
		Context:   ctxBuilder,
	})
	reg.Register(&agents.Executor{
		AgentID:   "agent-checker",
		AgentRole: core.RoleChecker,
		Provider:  prov,
		Context:   ctxBuilder,
	})
	reg.Register(&agents.Executor{
		AgentID:   "agent-recovery",
		AgentRole: core.RoleRecovery,
		Provider:  prov,
		Context:   ctxBuilder,
	})

	orch := pipeline.New(cfg, mem, bus, reg)

	input := &core.Input{
		TaskDescription: "Create a simple hello world script",
	}

	out, err := orch.Run(context.Background(), input)
	if err != nil {
		t.Fatalf("pipeline run failed: %v", err)
	}

	if out.Status != core.StatusSuccess {
		t.Fatalf("expected success, got %s", out.Status)
	}

	content, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("failed to read test file: %v", err)
	}

	t.Logf("hello.txt content: %s", string(content))
}

func TestPipelineRecovery(t *testing.T) {
	dir := t.TempDir()

	testFile := filepath.Join(dir, "broken.py")
	if err := os.WriteFile(testFile, []byte("print('before')"), 0644); err != nil {
		t.Fatal(err)
	}

	prov := &llm.MockProvider{FailCount: 1}
	ctxBuilder := &ctxbuilder2.Builder{MaxTokens: 32000}
	toolReg := tools.New()
	tools.RegisterFileTools(toolReg, dir)

	mem, err := memory.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}

	bus := events.NewInMemoryBus()
	cfg := &core.Config{WorkDir: dir}

	reg := agents.NewRegistry()
	reg.Register(&agents.Executor{
		AgentID:   "agent-parser",
		AgentRole: core.RoleParser,
		Provider:  prov,
		Context:   ctxBuilder,
	})
	reg.Register(&agents.Executor{
		AgentID:      "agent-developer",
		AgentRole:    core.RoleDeveloper,
		Provider:     prov,
		Context:      ctxBuilder,
		ToolRegistry: toolReg,
	})
	reg.Register(&agents.Executor{
		AgentID:   "agent-tester",
		AgentRole: core.RoleTester,
		Provider:  prov,
		Context:   ctxBuilder,
	})
	reg.Register(&agents.Executor{
		AgentID:   "agent-checker",
		AgentRole: core.RoleChecker,
		Provider:  prov,
		Context:   ctxBuilder,
	})
	reg.Register(&agents.Executor{
		AgentID:   "agent-recovery",
		AgentRole: core.RoleRecovery,
		Provider:  prov,
		Context:   ctxBuilder,
	})

	orch := pipeline.New(cfg, mem, bus, reg)

	input := &core.Input{
		TaskDescription: "Fix the broken script",
	}

	out, err := orch.Run(context.Background(), input)
	if err != nil {
		t.Fatalf("pipeline run failed: %v", err)
	}

	if out.Status != core.StatusSuccess {
		t.Fatalf("expected success after recovery, got %s", out.Status)
	}

	t.Logf("Pipeline completed with recovery: %s", out.Message)
}

func TestFileToolsWriteAndRead(t *testing.T) {
	dir := t.TempDir()

	toolReg := tools.New()
	tools.RegisterFileTools(toolReg, dir)

	ctx := context.Background()

	result, err := toolReg.Execute(ctx, core.ToolCall{
		Type: "function",
		Function: core.FunctionCall{
			Name:      "write_file",
			Arguments: `{"path": "test_output.txt", "content": "hello from test"}`,
		},
	})
	if err != nil {
		t.Fatalf("write_file failed: %v", err)
	}

	t.Logf("write_file result: %s", result)

	content, err := os.ReadFile(filepath.Join(dir, "test_output.txt"))
	if err != nil {
		t.Fatalf("file not created: %v", err)
	}
	if string(content) != "hello from test" {
		t.Fatalf("expected 'hello from test', got '%s'", string(content))
	}

	result, err = toolReg.Execute(ctx, core.ToolCall{
		Type: "function",
		Function: core.FunctionCall{
			Name:      "read_file",
			Arguments: `{"path": "test_output.txt"}`,
		},
	})
	if err != nil {
		t.Fatalf("read_file failed: %v", err)
	}

	t.Logf("read_file result: %s", result)
}

func TestPipelineEventsPublished(t *testing.T) {
	dir := t.TempDir()

	testFile := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(testFile, []byte("init"), 0644); err != nil {
		t.Fatal(err)
	}

	prov := &llm.MockProvider{}
	ctxBuilder := &ctxbuilder2.Builder{MaxTokens: 32000}
	toolReg := tools.New()
	tools.RegisterFileTools(toolReg, dir)

	mem, err := memory.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}

	bus := events.NewInMemoryBus()

	var mu sync.Mutex
	eventTypes := make(map[events.Type]int)
	recordHandler := func(ctx context.Context, event events.Event) error {
		mu.Lock()
		defer mu.Unlock()
		eventTypes[event.Type]++
		return nil
	}

	bus.Subscribe(events.TypePipelineStart, recordHandler)
	bus.Subscribe(events.TypePipelineEnd, recordHandler)
	bus.Subscribe(events.TypeAgentStart, recordHandler)
	bus.Subscribe(events.TypeAgentComplete, recordHandler)
	bus.Subscribe(events.TypeStateChange, recordHandler)

	cfg := &core.Config{WorkDir: dir}

	reg := agents.NewRegistry()
	reg.Register(&agents.Executor{AgentID: "agent-parser", AgentRole: core.RoleParser, Provider: prov, SystemPrompt: "You are an expert Parser.", Context: ctxBuilder})
	reg.Register(&agents.Executor{AgentID: "agent-developer", AgentRole: core.RoleDeveloper, Provider: prov, SystemPrompt: "You are an expert Developer.", Context: ctxBuilder, ToolRegistry: toolReg})
	reg.Register(&agents.Executor{AgentID: "agent-tester", AgentRole: core.RoleTester, Provider: prov, Context: ctxBuilder})
	reg.Register(&agents.Executor{AgentID: "agent-checker", AgentRole: core.RoleChecker, Provider: prov, Context: ctxBuilder})
	reg.Register(&agents.Executor{AgentID: "agent-recovery", AgentRole: core.RoleRecovery, Provider: prov, Context: ctxBuilder})

	orch := pipeline.New(cfg, mem, bus, reg)

	input := &core.Input{TaskDescription: "Create a hello world script"}
	out, err := orch.Run(context.Background(), input)
	if err != nil {
		t.Fatalf("pipeline run failed: %v", err)
	}
	if out.Status != core.StatusSuccess {
		t.Fatalf("expected success, got %s", out.Status)
	}

	mu.Lock()
	defer mu.Unlock()

	if eventTypes[events.TypePipelineStart] != 1 {
		t.Errorf("expected 1 pipeline_start event, got %d", eventTypes[events.TypePipelineStart])
	}
	if eventTypes[events.TypePipelineEnd] != 1 {
		t.Errorf("expected 1 pipeline_end event, got %d", eventTypes[events.TypePipelineEnd])
	}
	if eventTypes[events.TypeAgentStart] == 0 {
		t.Error("expected at least 1 agent_start event")
	}
}

func TestPipelineRollbackOnMaxRetries(t *testing.T) {
	dir := t.TempDir()

	testFile := filepath.Join(dir, "data.txt")
	originalContent := []byte("do not modify")
	if err := os.WriteFile(testFile, originalContent, 0644); err != nil {
		t.Fatal(err)
	}

	prov := &llm.MockProvider{FailCount: 999}
	ctxBuilder := &ctxbuilder2.Builder{MaxTokens: 32000}
	toolReg := tools.New()
	tools.RegisterFileTools(toolReg, dir)

	mem, err := memory.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}

	bus := events.NewInMemoryBus()
	cfg := &core.Config{WorkDir: dir}

	reg := agents.NewRegistry()
	reg.Register(&agents.Executor{AgentID: "agent-parser", AgentRole: core.RoleParser, Provider: prov, SystemPrompt: "You are the Lead Developer.", Context: ctxBuilder})
	reg.Register(&agents.Executor{AgentID: "agent-developer", AgentRole: core.RoleDeveloper, Provider: prov, SystemPrompt: "You are the Lead Developer. Coding Agent role.", Context: ctxBuilder, ToolRegistry: toolReg})
	reg.Register(&agents.Executor{AgentID: "agent-tester", AgentRole: core.RoleTester, Provider: prov, Context: ctxBuilder})
	reg.Register(&agents.Executor{AgentID: "agent-checker", AgentRole: core.RoleChecker, Provider: prov, Context: ctxBuilder})
	reg.Register(&agents.Executor{AgentID: "agent-recovery", AgentRole: core.RoleRecovery, Provider: prov, SystemPrompt: "You are the Recovery Agent.", Context: ctxBuilder})

	orch := pipeline.New(cfg, mem, bus, reg)

	input := &core.Input{TaskDescription: "Modify the data file"}
	out, err := orch.Run(context.Background(), input)

	if err == nil {
		t.Fatal("expected pipeline error after max retries")
	}
	if out != nil && out.Status == core.StatusSuccess {
		t.Fatal("expected failure status, got success")
	}

	content, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("failed to read test file: %v", err)
	}
	if string(content) != string(originalContent) {
		t.Errorf("expected rollback to restore original content, got %q", string(content))
	}
}

func TestPipelineCancellation(t *testing.T) {
	dir := t.TempDir()

	testFile := filepath.Join(dir, "watch.txt")
	if err := os.WriteFile(testFile, []byte("initial"), 0644); err != nil {
		t.Fatal(err)
	}

	prov := &llm.MockProvider{}
	ctxBuilder := &ctxbuilder2.Builder{MaxTokens: 32000}
	toolReg := tools.New()
	tools.RegisterFileTools(toolReg, dir)

	mem, err := memory.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}

	bus := events.NewInMemoryBus()

	var cancelled bool
	bus.Subscribe(events.TypeStateChange, func(ctx context.Context, event events.Event) error {
		if event.Type == events.TypeStateChange {
			if to, ok := event.Payload["to"].(string); ok && to == "cancelled" {
				cancelled = true
			}
		}
		return nil
	})

	cfg := &core.Config{WorkDir: dir}

	reg := agents.NewRegistry()
	reg.Register(&agents.Executor{AgentID: "agent-parser", AgentRole: core.RoleParser, Provider: prov, SystemPrompt: "You are an expert Parser.", Context: ctxBuilder})
	reg.Register(&agents.Executor{AgentID: "agent-developer", AgentRole: core.RoleDeveloper, Provider: prov, SystemPrompt: "You are an expert Developer.", Context: ctxBuilder, ToolRegistry: toolReg})
	reg.Register(&agents.Executor{AgentID: "agent-tester", AgentRole: core.RoleTester, Provider: prov, Context: ctxBuilder})
	reg.Register(&agents.Executor{AgentID: "agent-checker", AgentRole: core.RoleChecker, Provider: prov, Context: ctxBuilder})
	reg.Register(&agents.Executor{AgentID: "agent-recovery", AgentRole: core.RoleRecovery, Provider: prov, Context: ctxBuilder})

	orch := pipeline.New(cfg, mem, bus, reg)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan bool)
	go func() {
		_, _ = orch.Run(ctx, &core.Input{TaskDescription: "Create a file"})
		done <- true
	}()

	cancel()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("pipeline did not respond to cancellation within 5 seconds")
	}

	if !cancelled {
		t.Error("expected cancelled state event to be published")
	}
}
