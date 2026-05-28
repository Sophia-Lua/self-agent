# Development Guide

## Code Organization

```
autodev/
├── cmd/autodev/        # CLI entry points
├── internal/           # Private packages
│   ├── core/           # Shared types and interfaces
│   ├── pipeline/       # State machine orchestrator
│   ├── agents/         # Agent executors
│   ├── parser/         # Task decomposition
│   ├── session/        # Checkpoint management
│   ├── git/            # PR creation
│   ├── llm/            # LLM providers
│   ├── tools/          # Tool registry
│   ├── context/        # Context builder
│   └── ...             # Other modules
└── docs/               # Documentation
```

## Module Dependencies

```
cmd/autodev
    └── pipeline
        ├── agents
        │   ├── core
        │   ├── tools
        │   ├── context
        │   └── llm
        ├── parser
        ├── session
        ├── git
        ├── memory
        └── events
```

## Adding New Modules

### 1. Create Module Structure

```bash
mkdir -p internal/myfeature
touch internal/myfeature/myfeature.go
touch internal/myfeature/myfeature_test.go
```

### 2. Define Interfaces in `core/`

```go
// internal/core/types.go
type MyFeature interface {
    Process(input Input) (*Output, error)
}
```

### 3. Implement Module

```go
package myfeature

import "autodev/internal/core"

type impl struct {
    config Config
}

func New(config Config) *impl {
    return &impl{config: config}
}

func (i *impl) Process(input core.Input) (*core.Output, error) {
    // Implementation
    return &core.Output{Status: core.StatusSuccess}, nil
}
```

### 4. Write Tests

```go
package myfeature

import (
    "context"
    "testing"
    "autodev/internal/core"
)

func TestProcess(t *testing.T) {
    impl := New(Config{})
    
    input := core.Input{TaskDescription: "test"}
    output, err := impl.Process(input)
    
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if output.Status != core.StatusSuccess {
        t.Errorf("expected success, got %s", output.Status)
    }
}
```

## Testing Guidelines

### Unit Tests

```bash
# Test specific package
go test ./internal/agents/... -v

# Test with coverage
go test ./internal/parser/... -coverprofile=coverage.out
go tool cover -html=coverage.out

# Run with race detector
go test ./... -race
```

### Integration Tests

```go
func TestPipelineIntegration(t *testing.T) {
    dir := t.TempDir()
    
    // Setup
    cfg := &core.Config{WorkDir: dir}
    mem, _ := memory.New(":memory:")
    bus := events.NewInMemoryBus()
    reg := agents.NewRegistry()
    
    // Register agents
    reg.Register(&agents.Executor{...})
    
    // Execute
    orch := pipeline.New(cfg, mem, bus, reg)
    out, err := orch.Run(context.Background(), &core.Input{
        TaskDescription: "create hello.txt",
    })
    
    // Assert
    if err != nil {
        t.Fatal(err)
    }
}
```

### Mock Testing

```go
// Use MockProvider for LLM testing
prov := &llm.MockProvider{
    FailCount: 1,  // Simulate one failure for recovery testing
}
```

## Code Style

### Naming Conventions

| Type | Convention | Example |
|------|------------|---------|
| Package | lowercase | `internal/parser/` |
| Interface | -er suffix | `Provider`, `Executor` |
| Constructor | New | `NewProvider()` |
| Constants | PascalCase | `StateParsing` |
| Variables | camelCase | `maxRetries` |
| Errors | prefix module | `fmt.Errorf("parser: %w", err)` |

### Error Handling

```go
// Wrap errors with context
return fmt.Errorf("failed to save checkpoint: %w", err)

// Use sentinel errors for expected conditions
var ErrNotFound = errors.New("agent not found")

// Check with errors.Is
if errors.Is(err, ErrNotFound) {
    // Handle
}
```

### Context Usage

```go
// Always pass context
func (e *Executor) Execute(ctx context.Context, input core.Input) (*core.Output, error) {
    select {
    case <-ctx.Done():
        return nil, ctx.Err()
    default:
    }
    // ...
}
```

## Debugging

### Enable Verbose Logging

```bash
# Pipeline logs are printed by default
./autodev run "task" --dry-run
```

### Checkpoint Inspection

```bash
# List checkpoints
./autodev session list

# View checkpoint JSON
cat .autodev/checkpoints/session-123.json | jq
```

### Event Monitoring

Add event listener in code:

```go
bus.Subscribe(func(event events.Event) {
    fmt.Printf("[%s] %s: %+v\n", event.Type, event.Agent, event.Payload)
})
```

## Build & Release

### Makefile Targets

```bash
make build            # Normal build
make build_race       # Build with race detector
make test             # Run all tests
make test_cover       # Test with coverage
make clean            # Remove build artifacts
```

### Dependencies

```bash
# Add dependency
go get github.com/example/package

# Update all dependencies
go get -u ./...

# Tidy go.mod
go mod tidy
```

## Common Patterns

### Tool Registration

```go
toolReg := tools.New()
tools.RegisterFileTools(toolReg, workDir)

// Add custom tool
toolReg.Register(core.Tool{
    Name:        "my_tool",
    Description: "Does something",
    Parameters:  mySchema,
    Execute:     myHandler,
})
```

### Agent Registration

```go
reg := agents.NewRegistry()
reg.Register(&agents.Executor{
    AgentID:      "my-agent",
    AgentRole:    core.RoleDeveloper,
    AgentDesc:    "Description",
    Provider:     llmProvider,
    SystemPrompt: "You are...",
    ToolRegistry: toolReg,
    Context:      ctxBuilder,
})
```

### Checkpoint Usage

```go
// Create orchestrator with session
orch := pipeline.New(cfg, mem, bus, reg).
    WithSessionID("session-123").
    WithAutoCheckpoint(true)

// Resume later
sess := session.New(workDir)
input, state, history, retries, err := sess.ResumeSession("session-123")
```

## Testing Checklist

Before submitting code:

- [ ] Unit tests written for new functions
- [ ] Integration tests for pipeline changes
- [ ] All tests pass: `go test ./...`
- [ ] No race conditions: `go test -race ./...`
- [ ] Code compiles: `go build ./...`
- [ ] Lint clean (if linter configured)
- [ ] Documentation updated
- [ ] No hardcoded secrets
- [ ] Error messages are descriptive