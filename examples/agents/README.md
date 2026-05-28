# Example Agents for autodev

This directory contains example agent YAML definitions that can be loaded by the
autodev registry via the `--agents-dir` flag.

## Usage

```bash
autodev run "implement a REST API for user management" --agents-dir ./examples/agents
```

## Included Templates

| Agent | Role | Description |
|-------|------|-------------|
| `architect` | Architect | System design and architecture planning |
| `security-auditor` | Security | Security review and vulnerability scanning |
| `documentation-writer` | Documentation | Technical documentation generation |
| `code-reviewer-enhanced` | Reviewer | Enhanced code review with specific focus areas |
| `data-engineer` | Developer | Database and data pipeline development |

## Variable Resolution

Agents support Go template variables using `{{variable}}` syntax. Define them
in the YAML `variables` section and resolve them at load time.

Example:
```yaml
variables:
  project_name: my-service
  db_type: postgresql
```

## Custom Agents

Create your own agent YAML files in this directory. Each file must define:
- `id`: Unique identifier (e.g., `agent-architect`)
- `name`: Human-readable name
- `role`: Agent role (parser, developer, tester, checker, recovery, or custom)
- `description`: Brief description of capabilities
- `system_prompt`: The LLM system prompt that defines agent behavior
- `tools`: Optional list of tool names to enable for this agent
