# Pipeline State Machine

## Overview

The pipeline orchestrator implements a deterministic state machine that coordinates multiple AI agents through a development lifecycle.

## State Diagram

```
                    ┌─────────┐
                    │ pending │
                    └────┬────┘
                         │
                    ┌────▼────┐
               ┌───│ parsing │───┐
               │   └────┬────┘   │
               │        │        │
               │   ┌────▼────┐   │
               │   │developing│  │
               │   └────┬────┘   │
               │        │        │
               │   ┌────▼────┐   │
               │   │ testing  │  │
               │   └────┬────┘   │
               │        │        │
               │   ┌────▼────┐   │
               │   │ checking │  │
               │   └────┬────┘   │
               │        │        │
               │   ┌────▼──────┐ │
               └──▶│ recovering│─┘
                   └────┬──────┘
                        │
                   ┌────▼────┐
                   │rollback │
                   └────┬────┘
                        │
                   ┌────▼────┐
                   │completed│
                   └─────────┘
```

## State Definitions

### pending
**Initial state** - Task queued, waiting to start.
- No agent execution
- Checkpoint skipped

### parsing
**Task Analysis** - Parser agent analyzes requirements.
- Calls `parser.Decompose()` to break task into subtasks
- Generates structured subtask list with dependencies
- Appends task plan to input context
- Checkpoint saved after completion

**Agent:** agent-parser
**Next States:** developing (success), recovering (failure)

### developing
**Code Generation** - Developer agent writes/modifies code.
- Uses tools (read_file, write_file, execute_command)
- Tool call loop (max 10 iterations)
- ContextBuilder provides relevant files
- Checkpoint saved after completion

**Agent:** agent-developer
**Next States:** testing (success), recovering (failure)

### testing
**Test Validation** - Tester agent creates/runs tests.
- Uses testrunner multi-framework support
- Frameworks: Go Test, Jest, Vitest, Pytest, Cargo
- Extracts failure details
- Checkpoint saved after completion

**Agent:** agent-tester
**Next States:** checking (success), recovering (failure)

### checking
**Quality Gates** - Checker agent validates quality.
- Coverage parsing (Go, Python, JS)
- Static analysis integration:
  - golangci-lint
  - go vet
  - eslint
  - flake8
  - pylint
- Checkpoint saved after completion

**Agent:** agent-checker
**Next States:** completed (success), recovering (failure)

### recovering
**Error Recovery** - Recovery agent attempts fix.
- Receives error context and history
- Max retries: 3 (configurable)
- After successful recovery → developing state
- After failure or max retries → rollback

**Agent:** agent-recovery
**Next States:** developing (recovery success), rollback (max retries)

### completed
**Task Done** - Pipeline successfully finished.
- Saves to SQLite memory
- Auto PR creation (if configured)
- Final checkpoint saved
- Returns success output

### rollback
**Failure** - Pipeline failed, workspace restored.
- Restores from initial snapshot
- Returns error with context

### cancelled
**Interrupted** - Context cancelled.
- Published via event bus
- Returns ctx.Err()

## Execution Loop

```go
func (o *Orchestrator) Run(ctx context.Context, input *core.Input) (*core.Output, error) {
    o.transition(StateParsing)
    
    // Initial snapshot for rollback
    o.snapshot.CreateSnapshot(input.TaskDescription)
    
    for {
        select {
        case <-ctx.Done():
            o.transition(StateCancelled)
            return nil, ctx.Err()
        default:
        }
        
        // Refresh files before each step
        if o.state != StateParsing {
            input.Files = o.snapshot.CreateSnapshot("").Files
        }
        
        switch o.state {
        case StateParsing:
            o.decomposedTask = parser.Decompose(input.TaskDescription)
            // ... run parser agent
            o.saveCheckpoint(input)
            o.transition(StateDeveloping)
            
        case StateDeveloping:
            // ... run developer agent
            o.saveCheckpoint(input)
            o.transition(StateTesting)

        // ... other states
        
        case StateCompleted:
            o.saveCheckpoint(input)
            // Auto PR if configured
            if input.PRConfig.Enabled {
                o.createPR(input)
            }
            return &core.Output{Status: StatusSuccess}, nil
        }
    }
}
```

## Recovery Logic

```
Failure detected
    │
    ▼
Is retries >= maxRetries?
    │
    ├── Yes → rollback
    │
    └── No → Recovery Agent
                │
                ▼
         Recovery succeeds?
                │
                ├── Yes → Retry previous state
                │
                └── No → rollback
```

**Recovery Context:**
```
Previous task failed with error: {error}
Original Task: {task}
Please analyze this failure, suggest a fix, and update the files.
```

## Checkpoint Strategy

**When saved:**
- After parsing state completion
- After developing state completion
- After testing state completion
- After checking state completion
- After successful recovery
- On completion state

**Skipped:**
- pending state
- cancelled state
- When `autoCheckpoint` disabled

**Checkpoint Content:**
```json
{
  "session_id": "session-123",
  "state": "developing",
  "task_id": "...",
  "task_description": "...",
  "history": [
    {"role": "user", "content": "..."},
    {"role": "assistant", "content": "..."}
  ],
  "files": {"main.go": "package main..."},
  "created_at": "2026-05-24T...",
  "retry_count": 2,
  "last_snapshot_id": "snap-3"
}
```

## Configuration

```go
type Orchestrator struct {
    maxRetries     int  // Default: 3
    autoCheckpoint bool // Default: true
    sessionID      string
}
```

**Methods:**
- `WithSessionID(id)` - Set session identifier
- `WithAutoCheckpoint(enabled)` - Enable/disable auto-save