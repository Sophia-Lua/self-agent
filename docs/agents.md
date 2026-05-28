# Agent Configuration Guide

## Overview

AutoDev uses a registry of agents, each with a specific role in the development pipeline. Agents can be built-in (defined in code) or custom (loaded from YAML).

## Built-in Agents

| Agent ID | Role | Description |
|----------|------|-------------|
| agent-parser | parser | Parses user intent into structured development tasks |
| agent-developer | developer | Generates code based on structured tasks |
| agent-tester | tester | Validates code correctness |
| agent-checker | checker | Validates code quality, coverage, and standards compliance |
| agent-recovery | recovery | Attempts to fix task failures |

## Agent Roles

```go
type Role string

const (
    RoleParser   Role = "parser"
    RoleDeveloper Role = "developer"
    RoleTester   Role = "tester"
    RoleChecker  Role = "checker"
    RoleRecovery Role = "recovery"
    RoleCustom   Role = "custom"
)
```

## Custom Agent YAML

Create files in `./agents/` directory with `.yaml` or `.yml` extension.

### Schema

```yaml
id: string              # Required. Unique agent identifier
name: string            # Optional. Display name (fallback to id)
role: string            # Required. One of: parser, developer, tester, checker, recovery
description: string     # Optional. Agent description
provider: string        # Optional. LLM provider override
model: string           # Optional. LLM model override
max_tokens: int         # Optional. Maximum tokens (default: 4096)
max_tool_calls: int     # Optional. Max tool calls per turn (default: 10)
system_prompt: string   # Required. Agent's system prompt
temperature: float      # Optional. LLM temperature (0.0 - 2.0)
tools: [string]         # Optional. Allowed tool names
variables:              # Optional. Template variables
  key: value
```

### Example: Security Auditor

```yaml
id: security-auditor
name: Security Auditor
role: checker
description: Focuses on security vulnerabilities and best practices
system_prompt: |
  You are a senior security expert reviewing code for:
  - SQL injection vulnerabilities
  - Cross-site scripting (XSS) risks
  - Insecure dependency versions
  - Hardcoded credentials or API keys
  - Permission and authorization gaps
  
  Provide specific examples and remediation steps.
temperature: 0.3
max_tokens: 8192
```

### Example: Frontend Specialist

```yaml
id: frontend-dev
name: Frontend Developer
role: developer
description: Specializes in React/Vue component development
system_prompt: |
  You are a frontend expert specializing in {{.framework}}.
  
  Project: {{.project}}
  Environment: {{.environment}}
  
  Follow these guidelines:
  - Use functional components with hooks
  - Implement proper TypeScript types
  - Follow accessibility best practices
  - Add comprehensive tests
  
variables:
  framework: React
  project: my-app
  environment: production
tools:
  - read_file
  - write_file
  - execute_command
```

## Template Variables

System prompts support Go template syntax with `{{.variable}}` syntax.

### Available Functions

| Function | Description | Example |
|----------|-------------|---------|
| `default` | Fallback value | `{{default "unknown" .name}}` |
| `upper` | Uppercase | `{{upper .env}}` |
| `lower` | Lowercase | `{{lower .name}}` |
| `title` | Title case | `{{title .project}}` |
| `eq` | Equality check | `{{eq .env "production"}}` |
| `neq` | Inequality check | `{{neq .env "dev"}}` |
| `join` | Join slice | `{{join ", " .tags}}` |

### Variable Resolution Order

1. Agent YAML `variables` block
2. Loader-level injected variables via `WithVariables()`
3. Undefined variables show `<no value>` (non-strict mode)

### Template Syntax Options

Default delimiters: `{{` and `}}`

Custom delimiters in code:
```go
config := agents.TemplateConfig{
    Delimiters: [2]string{"[[", "]]"},
    Variables:  map[string]string{"name": "World"},
    StrictMode: false,
}
```

## Agent Loading

### From Directory

```go
loader := registry.New(reg)
if err := loader.LoadFromDir("./agents"); err != nil {
    // Handle error
}
```

### With Variables

```go
loader := registry.New(reg).WithVariables(map[string]string{
    "project": "my-app",
    "version": "1.0.0",
})
```

### Validation

```bash
# Validate all agents
./autodev agent validate all

# Validate specific agent
./autodev agent validate my-agent

# List loaded agents
./autodev agent list
```

## Agent Execution Flow

```
┌─────────────┐
│  Orchestrator│
└──────┬──────┘
       │ lookup agent by ID
       ▼
┌─────────────┐
│  Registry    │──── Returns Agent interface
└──────┬──────┘
       │
       ▼
┌─────────────┐
│  Executor    │
│  - Context   │──── Build input with files + history
│  - Tools     │──── Execute tool loop
│  - LLM       │──── Generate response
└──────┬──────┘
       │
       ▼
┌─────────────┐
│   Output     │── modified_files, status, message
└─────────────┘
```

## Custom Agent Execution

Custom agents loaded from YAML get a stub executor by default:

```go
// Default stub execution
return &core.Output{
    Status:  core.StatusSuccess,
    Message: fmt.Sprintf("Custom agent %s executed: %s", agentID, input.TaskDescription),
}, nil
```

To implement custom logic, extend the registry loader to bind to specific implementations.

## Best Practices

1. **One Agent Per File** - Each YAML file defines one agent
2. **Descriptive IDs** - Use kebab-case: `api-documentation-generator`
3. **Clear Roles** - Match role to agent responsibility
4. **Variable Usage** - Use variables for environment-specific prompts
5. **Tool Restrictions** - Limit tools to only what the agent needs
6. **Temperature Control** - Lower for deterministic tasks (0.2-0.5), higher for creative (0.7-1.0)

## Troubleshooting

### Agent Not Loaded
```
warning: failed to load agent my-agent.yaml: yaml: line 5: mapping values are not allowed
```
**Fix:** Check YAML indentation and syntax.

### Invalid Role
```
invalid role: reviewer (valid: parser, developer, tester, checker, recovery)
```
**Fix:** Use one of the valid role strings.

### Template Error
```
template parse error: ...
```
**Fix:** Check template syntax, ensure variables wrap in `{{.name}}`.
