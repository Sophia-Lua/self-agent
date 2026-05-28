# AutoDev

Autonomous AI-powered development agent that automates the complete software development lifecycle through multi-agent collaboration.

## Features

### Core Capabilities
- **Multi-Agent Pipeline**: State machine orchestrates Parser → Developer → Tester → Checker → Recovery agents
- **Smart Context Management**: Token estimation, file truncation, and priority-based context building
- **Automatic Task Decomposition**: Breaks complex tasks into structured subtasks with dependency chains
- **MCP Integration**: Seamless Model Context Protocol support for external tool servers
- **Multi-LLM Support**: OpenAI, Claude, Ollama (local), and Mock providers
- **Session Checkpoints**: Save and resume execution from any pipeline state
- **Auto PR Creation**: Automatic Pull/Merge Request creation on successful completion
- **Template Engine**: Dynamic variable resolution in agent prompts
- **Code Validation**: Coverage parsing and static analysis integration (golangci-lint, eslint, flake8, etc.)
- **Multi-Test Framework**: Go Test, Jest, Vitest, Pytest, Cargo test support

### Architecture Highlights
- **Tool Registry**: Unified execution layer for local functions and MCP tools
- **Event Bus**: Real-time pipeline state monitoring
- **SQLite Memory**: Persistent memory with FTS5 full-text search
- **AES-GCM Vault**: Secure secret encryption
- **Sandbox Executor**: Isolated command execution

## Quick Start

### Prerequisites
- Go 1.21+
- Access to LLM API (OpenAI, Anthropic) or local Ollama instance

### Build
```bash
make build
# or
go build -o autodev ./cmd/autodev
```

### Basic Usage
```bash
# Execute a development task
./autodev run "Add user registration endpoint with email validation"

# Dry-run mode (uses mock LLM for testing)
./autodev run "Fix login bug" --dry-run

# Resume from checkpoint
./autodev run "Refactor database layer" --resume session-123
```

## CLI Reference

### Main Commands
```
autodev run [task] [flags]              # Execute development task
autodev agent [subcommand]              # Manage custom agents
autodev session [subcommand]            # Manage checkpoints
autodev analyze [dir]                   # Project structure analysis
autodev diagnose [error]                # Error diagnosis
autodev encrypt [text] -p password      # AES-GCM encryption
autodev decrypt [cipher] -p password    # AES-GCM decryption
autodev sandbox [command]               # Sandboxed command execution
autodev status                          # Git repository status
autodev config                          # Show loaded configuration
```

### Run Flags
| Flag | Description | Default |
|------|-------------|---------|
| `--provider` | LLM provider (openai, claude, ollama, mock) | openai |
| `--model` | LLM model name | gpt-4o |
| `--api-key` | API key (or set env var) | - |
| `--agents-dir` | Custom agent YAML directory | ./agents |
| `--dry-run` | Use mock LLM for testing | false |
| `--fail-once` | Test recovery with one failure | false |
| `--mcp-config` | MCP servers config JSON file | - |
| `--resume` | Session ID to resume from | - |
| `--create-pr` | Auto-create PR after completion | false |
| `--pr-target` | Target branch for PR | main |
| `--pr-draft` | Create PR as draft | false |
| `--pr-reviewers` | Comma-separated reviewer list | - |

### Agent Subcommands
```bash
./autodev agent list                          # List all agents
./autodev agent create my-agent --role developer --system-prompt "..."
./autodev agent validate [agent-id|all]       # Validate YAML
./autodev agent show [agent-id]               # Display agent config
./autodev agent delete [agent-id]             # Remove agent
```

### Session Subcommands
```bash
./autodev session list                        # List checkpoints
./autodev session resume [session-id]         # Preview checkpoint
./autodev session delete [session-id]         # Remove checkpoint
```

## Pipeline States

```
pending → parsing → developing → testing → checking → completed
                 ↓                        ↓
              recovering ← max 3 retries  ↓
                 ↓                        ↓
              rollback ←──────────────────┘
```

Each state runs a specialized agent with role-specific prompts and tool access.

## Architecture

```
┌─────────────────────────────────────────────────────────┐
│                      CLI Layer                           │
│  run | agent | session | analyze | diagnose | encrypt    │
└───────────────────────────┬─────────────────────────────┘
                            │
┌───────────────────────────▼─────────────────────────────┐
│                   Pipeline Orchestrator                  │
│  State Machine │ Checkpoints │ Event Bus │ Auto-PR       │
└───────────────────────────┬─────────────────────────────┘
                            │
┌──────────┬──────────┬──────────┬──────────┬─────────────┐
│  Parser  │Developer │  Tester  │ Checker  │  Recovery   │
│  Agent   │  Agent   │  Agent   │  Agent   │   Agent     │
└──────────┴──────────┴──────────┴──────────┴─────────────┘
                            │
┌───────────────────────────┼─────────────────────────────┐
│                   Executor Core                          │
│  Context Builder │ Tool Registry │ LLM Providers         │
└───────────────────────────┼─────────────────────────────┘
                            │
┌──────────┬──────────┬──────────┬──────────┬─────────────┐
│ MCP      │Local     │SQLite    │File      │Sandbox      │
│ Client   │Tools     │Memory    │System    │Executor     │
└──────────┴──────────┴──────────┴──────────┴─────────────┘
```

## Custom Agents

Create agent YAML definitions in `./agents/`:

```yaml
id: security-auditor
name: Security Auditor
role: checker
description: Focuses on security vulnerabilities
system_prompt: |
  You are a security expert reviewing code for:
  - SQL injection risks
  - XSS vulnerabilities
  - Insecure dependencies
  - Hardcoded credentials
variables:
  project: my-app
  environment: production
```

Supported variables in system prompts use `{{.variable_name}}` syntax with functions:
- `{{default "fallback" .var}}` - Default values
- `{{upper .var}}` - Uppercase
- `{{lower .var}}` - Lowercase
- `{{title .var}}` - Title case

## MCP Integration

Configure MCP servers in JSON:

```json
[
  {
    "name": "filesystem",
    "command": "npx",
    "args": ["-y", "@modelcontextprotocol/server-filesystem", "/workspace"]
  }
]
```

Run with:
```bash
./autodev run "task" --mcp-config mcp.json
```

## Environment Variables

| Variable | Description |
|----------|-------------|
| `OPENAI_API_KEY` | OpenAI API key |
| `ANTHROPIC_API_KEY` | Anthropic API key |
| `GITHUB_TOKEN` | GitHub token for PR creation |
| `GITLAB_TOKEN` | GitLab token for MR creation |
| `GIT_TOKEN` | Fallback token for unknown platforms |

## Development

```bash
# Run tests
make test

# Build with race detection
make build_race

# Run specific tests
go test ./internal/pipeline/... -v
go test ./internal/agents/... -v

# Check build
go build ./...
```

## Test Coverage

| Module | Tests | Coverage |
|--------|-------|----------|
| `internal/agents` | Template rendering, variable resolution | 14 tests |
| `internal/parser` | Task decomposition, dependency chains | 18 tests |
| `internal/session` | Checkpoints, resume, auto-save | 10 tests |
| `internal/git` | PR creation, platform detection | 13 tests |
| `internal/pipeline` | State machine, recovery, git ops | 7 tests |
| `internal/context` | Token estimation, file truncation | 5 tests |

**Total: 62 tests passing**

## Project Structure

```
autodev/
├── cmd/autodev/          # CLI entry point and commands
│   ├── main.go           # Root command and flags
│   ├── agent.go          # Agent management commands
│   └── session.go        # Session management commands
├── internal/
│   ├── agents/           # Agent executors and templates
│   ├── approval/         # User approval workflow
│   ├── audit/            # Audit logging
│   ├── concurrency/      # Concurrency control groups
│   ├── config/           # Viper YAML configuration
│   ├── context/          # Context builder with token limits
│   ├── core/             # Interfaces and types
│   ├── crypto/           # AES-GCM encryption
│   ├── diagnosis/        # Error pattern matching
│   ├── events/           # Event bus system
│   ├── git/              # PR creation and platform detection
│   ├── llm/              # LLM providers (OpenAI, Claude, Ollama, Mock)
│   ├── mcp/              # MCP client and stdio transport
│   ├── memory/           # SQLite memory with FTS5
│   ├── parser/           # Task decomposition engine
│   ├── pipeline/         # State machine orchestrator
│   ├── progress/         # Progress tracking
│   ├── project/          # Project structure analyzer
│   ├── registry/         # Agent YAML loader
│   ├── router/           # Agent routing logic
│   ├── sandbox/          # Sandboxed execution
│   ├── session/          # Checkpoint management
│   ├── subagent/         # Sub-agent task delegation
│   ├── testrunner/       # Multi-framework test executor
│   ├── timeout/          # Timeout policies
│   ├── tools/            # Tool registry and file operations
│   └── validator/        # Coverage and lint validation
└── Makefile
```

## License

MIT
