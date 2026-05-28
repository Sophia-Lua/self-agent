# Configuration Reference

## Overview

AutoDev uses Viper for configuration loading with layered priority:

1. Environment variables (highest)
2. Command line flags
3. Config file
4. Defaults (lowest)

## Config File

Default location: `./autodev.yaml`

### Complete Example

```yaml
# Working directory for file operations
work_dir: .

# Data directory for internal state
data_dir: ./.autodev

# SQLite database for memory (":memory:" for ephemeral)
memory_db: ":memory:"

# Default LLM provider
provider: openai

# Default model
model: gpt-4o

# Session configuration
session:
  db_path: ".autodev/sessions.db"
  auto_save: true

# MCP server configuration
mcp:
  servers:
    - name: filesystem
      command: npx
      args: ["-y", "@modelcontextprotocol/server-filesystem", "/workspace"]

# Pull Request creation config
pr_config:
  enabled: false
  platform: github
  target_branch: main
  labels:
    - auto-generated
  reviewers: []
  draft: false
```

## Environment Variables

| Variable | Type | Description |
|----------|------|-------------|
| `AUTODEV_WORK_DIR` | string | Working directory |
| `AUTODEV_DATA_DIR` | string | Data directory |
| `AUTODEV_MEMORY_DB` | string | SQLite path |
| `AUTODEV_PROVIDER` | string | LLM provider |
| `AUTODEV_MODEL` | string | Default model |
| `OPENAI_API_KEY` | string | OpenAI API key |
| `ANTHROPIC_API_KEY` | string | Anthropic API key |
| `GITHUB_TOKEN` | string | GitHub PAT for PR |
| `GITLAB_TOKEN` | string | GitLab PAT for MR |
| `GIT_TOKEN` | string | Fallback token |

## Provider Configuration

### OpenAI

```bash
export OPENAI_API_KEY=sk-...
./autodev run "task" --provider openai --model gpt-4o
```

### Claude

```bash
export ANTHROPIC_API_KEY=sk-ant-...
./autodev run "task" --provider claude --model claude-sonnet-4-20250514
```

### Ollama (Local)

```bash
# No API key needed
./autodev run "task" --provider ollama --model llama3
```

### Mock (Testing)

```bash
./autodev run "task" --dry-run
# Simulate one failure for recovery testing
./autodev run "task" --dry-run --fail-once
```

## Session Configuration

```yaml
session:
  db_path: ".autodev/sessions.db"
  auto_save: true
```

**Options:**
- `db_path` - SQLite database location
- `auto_save` - Enable automatic checkpoints

## MCP Configuration

```yaml
mcp:
  servers:
    - name: filesystem
      command: npx
      args:
        - "-y"
        - "@modelcontextprotocol/server-filesystem"
        - "/workspace"
    - name: database
      command: python
      args:
        - "-m"
        - "mcp_server_sqlite"
        - "./data.db"
```

**CLI override:**
```bash
./autodev run "task" --mcp-config mcp.json
```

## PR Configuration

```yaml
pr_config:
  enabled: true
  platform: github
  owner: my-org
  repo: my-repo
  target_branch: main
  labels:
    - feature
    - auto-generated
  reviewers:
    - dev1
    - dev2
  draft: false
```

**CLI override:**
```bash
./autodev run "task" \
  --create-pr \
  --pr-target main \
  --pr-draft \
  --pr-reviewers "dev1,dev2"
```

## Agent Directory

```yaml
# Default: ./agents
agents_dir: /path/to/agents
```

## Timeout Configuration

```yaml
# Default timeouts (seconds)
timeout:
  llm_request: 120
  tool_execution: 60
  command: 300
```

## Best Practices

1. **Use environment variables for secrets** - Never commit API keys
2. **Use config file for structure** - MCP servers, PR settings
3. **Use CLI flags for overrides** - Provider, model selection
4. **Test with mock provider** - `--dry-run` for safe testing
5. **Enable checkpoints** - Default behavior, don't disable
