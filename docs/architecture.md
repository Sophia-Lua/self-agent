# Architecture Design

## System Overview

AutoDev is a multi-agent autonomous development system that coordinates AI agents through a state machine pipeline to complete software development tasks.

## Core Components

### 1. Pipeline Orchestrator (`internal/pipeline/`)

The central coordinator that manages the state machine execution flow.

**Responsibilities:**
- State transitions and event publishing
- Agent execution coordination
- Checkpoint management
- Recovery logic (max 3 retries)
- Automatic PR creation on completion

**State Flow:**
```
pending → parsing → developing → testing → checking → completed
                 ↓                        ↓
              recovering ← max retries   ↓
                 ↓                        ↓
              rollback ←──────────────────┘
```

**Key Methods:**
- `Run(ctx, input)` - Main execution loop
- `runAgent(ctx, agentID, input)` - Execute specific agent
- `transition(state)` - State change with event publishing
- `saveCheckpoint(input)` - Auto-save execution state
- `createPR(input)` - Create pull request post-completion
- `getRemoteURL()` - Git remote detection via `git remote get-url`
- `getCurrentBranch()` - Current branch via `git rev-parse`

### 2. Agent Executor (`internal/agents/`)

Shared execution engine for all agent roles.

**Features:**
- LLM provider abstraction
- Tool call loop (max `MaxToolCalls` iterations)
- History management
- Context building via ContextBuilder
- Template variable resolution in system prompts

**Built-in Agents:**
| Agent ID | Role | Responsibility |
|----------|------|----------------|
| agent-parser | parser | Task analysis and decomposition |
| agent-developer | developer | Code generation and modification |
| agent-tester | tester | Test creation and validation |
| agent-checker | checker | Quality and coverage validation |
| agent-recovery | recovery | Error analysis and fix attempts |

### 3. Parser Engine (`internal/parser/`)

Intelligent task decomposition system.

**Decomposition Types:**
- `TypeFeature` - New feature implementation
- `TypeFix` - Bug fixing
- `TypeRefactor` - Code restructuring
- `TypeTest` - Test development
- `TypeConfig` - Configuration setup
- `TypeDoc` - Documentation updates

**Output:**
- `DecomposedTask` with structured subtasks
- Dependency chain between subtasks
- Agent assignment per subtask
- Priority ordering

**Functions:**
- `Decompose(taskDesc)` - Break task into subtasks
- `ValidateTask(dt)` - Verify task validity
- `MergeSubTasks(...)` - Combine multiple tasks
- `SummarizeTaskPlan(dt)` - Human-readable summary
- `ExtractFilesFromTask(desc)` - Parse file references

### 4. Context Builder (`internal/context/`)

Intelligent context assembly for LLM inputs.

**Algorithm:**
1. Collect all relevant files
2. Sort by priority (Config > Source > Doc > Test)
3. Estimate tokens using 4:1 char-to-token ratio
4. Truncate oversized files from lowest priority first
5. Assemble final prompt with history and context

**Configuration:**
- `MaxTokens` - Maximum context size (default: 32000)

### 5. Tool Registry (`internal/tools/`)

Unified execution layer for tools.

**Local Tools:**
- `read_file` - Read file content
- `write_file` - Write/create files
- `search_files` - Grep search
- `list_files` - Directory listing
- `execute_command` - Shell execution

**MCP Integration:**
- Load tools from external MCP servers
- Stdio transport for communication
- Unified interface via mcp_adapter.go

### 6. LLM Providers (`internal/llm/`)

Multi-provider abstraction layer.

**Supported Providers:**
| Provider | File | Features |
|----------|------|----------|
| OpenAI | openai.go | GPT models, function calling |
| Claude | claude.go | Anthropic models |
| Ollama | ollama.go | Local models, no API key |
| Mock | mock.go | Testing, fail-once simulation |

**Provider Interface:**
```go
type Provider interface {
    Generate(ctx, prompt, history) (*Response, error)
}
```

### 7. Session Manager (`internal/session/`)

Checkpoint and resume functionality.

**Features:**
- File-based checkpoint storage
- JSON serialization of execution state
- Auto-checkpoint on state transitions
- Session resume from any checkpoint

**Storage:** `.autodev/checkpoints/<session-id>.json`

**Key Methods:**
- `SaveCheckpoint(sessionID, state, input, history, retryCount)`
- `LoadCheckpoint(sessionID)`
- `ResumeSession(sessionID)` - Returns input ready to continue
- `ListCheckpoints()` - All saved checkpoints
- `DeleteCheckpoint(sessionID)`
- `AutoCheckpoint(...)` - Smart save (skips pending/cancelled)
- `ResumeFromAny()` - Most recent checkpoint
- `CreateSnapshot(taskID)` - File system snapshot
- `RestoreSnapshot(snapshot)` - File system restore

### 8. Git PR Client (`internal/git/`)

Automatic PR/MR creation.

**Supported Platforms:**
- GitHub (REST API v3)
- GitLab (API v4)
- Bitbucket (API v2.0)

**Auto-Detection:**
- URL pattern matching for platform detection
- Owner/repo extraction from SSH and HTTPS URLs
- Token priority: config > GITHUB_TOKEN > GITLAB_TOKEN > GIT_TOKEN

**Features:**
- PR/MR creation with title and description
- Label assignment
- Reviewer assignment
- Draft mode support
- LLM-generated descriptions from changes

### 9. Configuration (`internal/config/`)

Viper-based YAML configuration loader.

**Config Structure:**
```yaml
work_dir: .
data_dir: ./.autodev
memory_db: ":memory:"
provider: openai
model: gpt-4o
```

**PR Configuration:**
```yaml
pr_config:
  enabled: true
  platform: github
  target_branch: main
  labels: [feature, auto-generated]
  reviewers: [dev1, dev2]
  draft: false
```

### 10. Event Bus (`internal/events/`)

Real-time event system for pipeline monitoring.

**Event Types:**
- `TypeStateChange` - Pipeline state transitions
- `TypeAgentStart` - Agent execution begins
- `TypeAgentComplete` - Agent execution finished
- `TypeAgentError` - Agent execution failed

### 11. Memory Store (`internal/memory/`)

SQLite-based persistent memory.

**Features:**
- FTS5 full-text search
- Session-scoped storage
- Save/retrieve key-value pairs

### 12. Additional Modules

| Module | Purpose |
|--------|---------|
| `approval/` | User confirmation workflow |
| `audit/` | Audit log with CSV export |
| `concurrency/` | Concurrency control groups |
| `crypto/` | AES-GCM encryption vault |
| `diagnosis/` | Error pattern matching |
| `progress/` | Progress tracking |
| `project/` | Project structure analyzer |
| `router/` | Agent routing decisions |
| `sandbox/` | Isolated command execution |
| `subagent/` | Task delegation |
| `testrunner/` | Multi-test framework executor |
| `timeout/` | Timeout policies |
| `validator/` | Coverage and lint gates |

## Data Flow

```
User Input
    │
    ▼
┌─────────────┐
│  CLI Layer  │── flags, args, env vars
└──────┬──────┘
       │
       ▼
┌─────────────┐
│   Parser    │── decomposes into subtasks
└──────┬──────┘
       │
       ▼
┌─────────────┐     ┌──────────────┐
│ Orchestrator│────▶│ Checkpoints  │
│ State Loop  │     │ (file-based) │
└──────┬──────┘     └──────────────┘
       │
       ▼
┌─────────────┐
│   Agent     │── Executor with LLM + Tools
│  Executor   │
└──────┬──────┘
       │
       ├──▶ Context Builder ──▶ File System
       ├──▶ Tool Registry ────▶ Local/MCP Tools
       ├──▶ LLM Provider ─────▶ OpenAI/Claude/Ollama
       └──▶ History Management
       
       ▼
┌─────────────┐
│   Output    │── modified files, PR info
└─────────────┘
```

## Error Handling Strategy

1. **Agent Failure** → Recovery Agent (max 3 retries)
2. **Recovery Failure** → Rollback to last snapshot
3. **LLM Provider Error** → Propagate to orchestrator
4. **Tool Execution Error** → Return to LLM for resolution
5. **Checkpoint Save Error** → Log warning, continue
6. **PR Creation Error** → Log warning, mark pipeline completed

## Security Considerations

- AES-GCM encryption for secrets in Vault
- Sandboxed command execution with isolated workspace
- Token-based API key management (no hardcoded keys)
- PR tokens from environment variables only
- File operation restrictions via tool registry
- Timeout policies to prevent resource exhaustion
