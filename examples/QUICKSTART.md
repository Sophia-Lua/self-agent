# Quickstart

Get autodev up and running in minutes.

## Prerequisites

- Go 1.21 or later
- OpenAI API key (or Claude/Ollama)
- Git repository

## Installation

```bash
go build -o autodev ./cmd/autodev
```

Or add to your PATH:

```bash
go install ./cmd/autodev
```

## Basic Usage

Run autodev with a task description:

```bash
autodev run "add JWT authentication to the user API" --provider openai
```

Use the mock provider to test workflow without calling LLM APIs:

```bash
autodev run "implement a login endpoint" --dry-run
```

## Configuration

Copy an example config and customize:

```bash
cp examples/autodev.yaml autodev.yaml
```

Then set your API key:

```bash
export OPENAI_API_KEY="your-key-here"
autodev run "..."
```

## Custom Agents

Load custom agents from YAML definitions:

```bash
autodev run "..." --agents-dir ./examples/agents
```

See `examples/agents/README.md` for available templates.

## Webhook Notifications

Enable webhooks in `autodev.yaml` to receive real-time notifications:

```yaml
webhook:
  enabled: true
  urls:
    - https://your-webhook-url
```

## Full Pipeline

The default pipeline runs these agents in sequence:

1. **Parser** - Decomposes the task into subtasks
2. **Developer** - Implements the solution
3. **Tester** - Writes and runs tests
4. **Checker** - Reviews code quality

Failed steps trigger the **Recovery** agent, which attempts to fix the issue
before retrying. After 3 failed attempts, the pipeline rolls back.

## Commands

| Command | Description |
|---------|-------------|
| `autodev run [task]` | Execute a development task |
| `autodev status` | Show current pipeline state |
| `autodev session list` | List saved sessions |
| `autodev config` | Show loaded configuration |
| `autodev encrypt [text]` | Encrypt sensitive text |
| `autodev decrypt [text]` | Decrypt encrypted text |

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--provider` | openai | LLM provider (openai, claude, ollama, mock) |
| `--model` | gpt-4o | LLM model name |
| `--api-key` | env | API key (falls back to OPENAI_API_KEY) |
| `--dry-run` | false | Use mock LLM provider |
| `--agents-dir` | ./agents | Custom agents directory |
| `--resume` | "" | Resume from session checkpoint |
| `--create-pr` | false | Auto-create GitHub/GitLab PR |
