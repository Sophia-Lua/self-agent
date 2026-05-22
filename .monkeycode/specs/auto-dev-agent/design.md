# AutoDev Agent 技术设计文档 (Go 语言实现)

## 1. 架构概览

### 1.1 整体架构

```
┌────────────────────────────────────────────────────────────────────┐
│                            CLI Interface                             │
│                    (cobra command parser & router)                   │
└──────────────────────────────┬─────────────────────────────────────┘
                               │
┌──────────────────────────────▼─────────────────────────────────────┐
│                        Task Orchestrator                             │
│           (Pipeline Management & State Machine & Agent Router)       │
└─────┬──────────┬──────────┬──────────┬──────────┬────────┬─────────┘
      │          │          │          │          │        │
┌─────▼─────┐ ┌──▼────┐ ┌───▼────┐ ┌───▼────┐ ┌──▼────┐ ┌──▼──────┐
│  Parser   │ │ Coder │ │ Tester │ │Checker │ │Memory │ │ Recovery │
│  Agent    │ │ Agent │ │ Agent  │ │ Agent  │ │System │ │  Agent   │
└─────┬─────┘ └───┬───┘ └───┬────┘ └───┬────┘ └───┬───┘ └────┬────┘
      │           │          │           │          │           │
┌─────▼───────────▼──────────▼───────────▼──────────▼───────────▼─────┐
│                      Context Manager                                  │
│        (Session Storage, File Cache, Git Operations, MCP Tools)       │
└──────┬────────────────────────┬──────────────────────────────┬───────┘
       │                        │                              │
┌──────▼───────┐        ┌───────▼───────┐               ┌──────▼──────┐
│ LLM Provider │        │ Session Store │               │  MCP Client │
│  (OpenAI,    │        │  (SQLite/     │               │  (Tools &   │
│   Claude,    │        │   Redis)      │               │  Resources) │
│   Local)     │        │               │               │             │
└──────────────┘        └───────────────┘               └─────────────┘
```

### 1.2 核心模块

| 模块 | 职责 | 关键能力 |
|------|------|---------|
| CLI Interface | 命令行交互 | Cobra 参数解析、进度展示、用户交互 |
| Task Orchestrator | 任务编排 | 状态管理、Agent 路由、流程控制、重试逻辑 |
| Parser Agent | 任务解析 | NLP 解析、任务分解、计划生成、子 Agent 调度 |
| Coder Agent | 代码生成 | 代码生成、代码修改、代码审查、MCP Tool 调用 |
| Tester Agent | 测试执行 | 测试生成、测试运行、结果分析 |
| Checker Agent | 验收检查 | 覆盖率检查、静态分析、质量门禁 |
| Recovery Agent | 恢复处理 | 自动修复、回退管理、重试策略 |
| Memory System | 记忆管理 | 用户指令记忆、项目知识存储、上下文检索 |
| Context Manager | 上下文管理 | 文件缓存、Git 操作、会话持久化 |
| Session Store | 会话存储 | 任务状态持久化、历史记录、断点续传 |
| MCP Client | MCP 工具 | Tools 调用、Resources 访问、Prompts 使用 |

### 1.3 Go 技术栈

```yaml
语言: Go 1.21+
核心依赖:
  CLI框架: github.com/spf13/cobra
  配置管理: github.com/spf13/viper
  日志系统: github.com/sirupsen/logrus 或 go.uber.org/zap
  数据库: 
    - github.com/mattn/go-sqlite3 (SQLite 会话存储)
    - github.com/redis/go-redis/v9 (可选 Redis 缓存)
  Git操作: github.com/go-git/go-git/v5
  HTTP客户端: github.com/resty/resty/v2
  JSON处理: encoding/json + github.com/json-iterator/go
  并发控制: sync + golang.org/x/sync/errgroup
  状态机: 内置实现或 github.com/looplab/fsm
  MCP协议: 自定义实现 (基于 JSON-RPC 2.0)
```

## 2. 多 Agent 协同架构

### 2.1 Agent 接口与数据模型 (`core`)

基于 `internal/core/agent.go` 的设计实现。

```go
package core

// Role 定义了 Agent 在流水线中的职责
type Role string

const (
    RoleParser   Role = "parser"
    RoleDeveloper Role = "developer"
    RoleTester   Role = "tester"
    RoleChecker  Role = "checker"
    RoleRecovery Role = "recovery"
    RoleCustom   Role = "custom"
)

// Input 所有 Agent 执行的标准输入
type Input struct {
    TaskDescription string            `json:"task_description"`
    Context         string            `json:"context,omitempty"`
    History         []Message         `json:"history,omitempty"`
    Files           map[string]string `json:"files,omitempty"` // 路径 -> 内容
    Config          map[string]any    `json:"config,omitempty"` // Agent 级覆盖配置
    MaxRetries      int               `json:"max_retries,omitempty"`
}

// Output Agent 执行的标准化结果
type Output struct {
    Status       ExecutionStatus   `json:"status"`
    Message      string            `json:"message,omitempty"`
    ModifiedFiles map[string]string `json:"modified_files,omitempty"` // 路径 -> 新内容
    NextState    PipelineState     `json:"next_state,omitempty"`     // 建议的状态流转
    Data         map[string]any    `json:"data,omitempty"`           // 任意结构化数据
    Error        error             `json:"-"`
}

// Agent 接口定义
type Agent interface {
    ID() string
    Role() Role
    Description() string
    Execute(ctx context.Context, input Input) (*Output, error)
}
```

### 2.2 Agent 注册表与执行器 (`agents`)

支持内置 Agent 注册与自定义 Agent 绑定。

```go
package agents

// Registry 管理所有已注册的 Agent
type Registry struct {
    mu     sync.RWMutex
    agents map[string]core.Agent
}

func (r *Registry) Register(agent core.Agent) error {
    // 线程安全的注册逻辑
}

// Executor 是内置的 LLM 驱动 Agent 实现
// 允许动态绑定 Prompt 和 Provider
type Executor struct {
    AgentID      string
    AgentRole    core.Role
    AgentDesc    string
    Provider     llm.Provider
    SystemPrompt string
}

func (e *Executor) Execute(ctx context.Context, input core.Input) (*core.Output, error) {
    // 1. 构建 Prompt (System Prompt + Task + Context)
    // 2. 调用 Provider.Chat()
    // 3. 解析 LLM 响应并返回 Output
}
```

### 2.2 Agent 协同机制

```go
package orchestrator

// AgentOrchestrator Agent 编排器
type AgentOrchestrator struct {
    agents      map[AgentType]Agent
    router      *AgentRouter
    memory      *MemorySystem
    session     *SessionStore
    mcpClient   *MCPClient
    stateMachine *StateMachine
}

// ExecuteWithAgents 多 Agent 协同执行
func (o *AgentOrchestrator) ExecuteWithAgents(
    ctx context.Context,
    task *Task,
) (*Result, error) {
    // 1. 从记忆系统获取历史上下文
    history := o.memory.GetTaskHistory(task.ID)
    
    // 2. 恢复会话状态（如果存在）
    session, err := o.session.GetSession(task.ID)
    if err == nil {
        task = session.RestoreTask()
    }
    
    // 3. 根据任务类型路由到合适的 Agent
    primaryAgent := o.router.Route(task)
    
    // 4. 执行主 Agent
    result, err := primaryAgent.Execute(ctx, task)
    if err != nil {
        // 失败时调用 Recovery Agent
        recoveryAgent := o.agents[AgentTypeRecovery]
        result, err = recoveryAgent.Execute(ctx, task)
        if err != nil {
            // 记录失败到记忆系统
            o.memory.RecordFailure(task.ID, err)
            return nil, err
        }
    }
    
    // 5. 保存执行结果到会话存储
    o.session.SaveSession(task.ID, task, result)
    
    // 6. 更新记忆系统
    o.memory.RecordSuccess(task.ID, result)
    
    return result, nil
}

// SpawnSubAgent 创建子 Agent 处理子任务
func (o *AgentOrchestrator) SpawnSubAgent(
    ctx context.Context,
    parentAgent Agent,
    subTask *Task,
) (*Result, error) {
    // 根据子任务类型选择合适的 Agent
    subAgent := o.router.Route(subTask)
    
    // 子 Agent 共享父 Agent 的记忆和会话
    subAgent.SetMemory(parentAgent.GetMemory())
    subAgent.SetSession(parentAgent.GetSession())
    
    // 执行子任务
    result, err := subAgent.Execute(ctx, subTask)
    if err != nil {
        return nil, fmt.Errorf("sub-agent %s failed: %w", subAgent.Type(), err)
    }
    
    return result, nil
}
```

### 2.3 Agent 路由策略

```go
package router

// AgentRouter Agent 路由器
type AgentRouter struct {
    agents     map[AgentType]Agent
    strategies []RoutingStrategy
}

// RoutingStrategy 路由策略接口
type RoutingStrategy interface {
    Match(task *Task) AgentType
    Priority() int
}

// TaskTypeRoutingStrategy 基于任务类型的路由
type TaskTypeRoutingStrategy struct{}

func (s *TaskTypeRoutingStrategy) Match(task *Task) AgentType {
    switch task.Type {
    case TaskTypeParse:
        return AgentTypeParser
    case TaskTypeCode:
        return AgentTypeCoder
    case TaskTypeTest:
        return AgentTypeTester
    case TaskTypeCheck:
        return AgentTypeChecker
    case TaskTypeFix:
        return AgentTypeRecovery
    default:
        return AgentTypeParser // 默认路由到 Parser
    }
}

// ComplexityRoutingStrategy 基于任务复杂度的路由
type ComplexityRoutingStrategy struct{}

func (s *ComplexityRoutingStrategy) Match(task *Task) AgentType {
    // 高复杂度任务可能需要多个 Agent 协同
    if task.Complexity > ComplexityHigh {
        // 复杂任务先经过 Parser Agent 分解
        return AgentTypeParser
    }
    
    // 低复杂度任务直接路由到具体 Agent
    return s.selectDirectAgent(task)
}

// Route 路由任务到合适的 Agent
func (r *AgentRouter) Route(task *Task) Agent {
    // 按优先级应用路由策略
    sort.Slice(r.strategies, func(i, j int) bool {
        return r.strategies[i].Priority() > r.strategies[j].Priority()
    })
    
    for _, strategy := range r.strategies {
        agentType := strategy.Match(task)
        if agent := r.agents[agentType]; agent != nil && agent.CanHandle(task) {
            return agent
        }
    }
    
    // 默认返回 Parser Agent
    return r.agents[AgentTypeParser]
}
```

## 3. 记忆模块设计

基于 `internal/memory/memory.go` 和 `internal/core/interfaces.go`。

### 3.1 记忆存储实现

使用 SQLite 提供轻量级的持久化，包含 FTS5 支持。

```go
package memory

// Store 实现了上下文和历史记录的持久化
type Store struct {
    db *sql.DB
}

// New 创建并初始化 SQLite 存储
func New(dsn string) (*Store, error) {
    db, err := sql.Open("sqlite3", dsn)
    if err != nil {
        return nil, err
    }
    
    if err := db.Ping(); err != nil {
        return nil, err
    }
    
    if err := initSchema(db); err != nil {
        return nil, err
    }
    
    return &Store{db: db}, nil
}

func initSchema(db *sql.DB) error {
    query := `
    CREATE TABLE IF NOT EXISTS memory (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        task_id TEXT NOT NULL,
        key TEXT NOT NULL,
        value TEXT NOT NULL,
        embedding BLOB, -- 预留给向量检索
        created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
    );
    CREATE INDEX IF NOT EXISTS idx_memory_task ON memory(task_id);
    CREATE INDEX IF NOT EXISTS idx_memory_key ON memory(key);
    `
    _, err := db.Exec(query)
    return err
}

// Save 存储与任务关联的键值对
func (s *Store) Save(ctx context.Context, taskID, key, value string) error {
    _, err := s.db.ExecContext(ctx, 
        "INSERT INTO memory (task_id, key, value) VALUES (?, ?, ?)", 
        taskID, key, value)
    return err
}

// Load 检索最后一次存储的值
func (s *Store) Load(ctx context.Context, taskID, key string) (string, error) {
    var value string
    err := s.db.QueryRowContext(ctx, 
        "SELECT value FROM memory WHERE task_id = ? AND key = ? ORDER BY created_at DESC LIMIT 1", 
        taskID, key).Scan(&value)
    if err == sql.ErrNoRows {
        return "", nil
    }
    return value, err
}
```

### 3.2 MemoryProvider 接口

定义在 `core` 包中，供 Agent 调用。

```go
package core

type MemoryProvider interface {
    SaveContext(ctx context.Context, taskID string, key string, value string) error
    LoadContext(ctx context.Context, taskID string, key string) (string, error)
    SearchMemory(ctx context.Context, query string, limit int) ([]MemoryResult, error)
}
```

### 3.2 记忆存储实现

```go
package memory

// MemoryStore 记忆存储接口
type MemoryStore interface {
    Save(entry *MemoryEntry) error
    Get(id string) (*MemoryEntry, error)
    GetByTaskID(taskID string) []*MemoryEntry
    GetByType(type MemoryType) []*MemoryEntry
    Delete(id string) error
    List() []*MemoryEntry
}

// SQLiteMemoryStore SQLite 存储实现
type SQLiteMemoryStore struct {
    db *sql.DB
}

func NewSQLiteMemoryStore(dbPath string) (*SQLiteMemoryStore, error) {
    db, err := sql.Open("sqlite3", dbPath)
    if err != nil {
        return nil, err
    }
    
    // 创建表
    schema := `
    CREATE TABLE IF NOT EXISTS memories (
        id TEXT PRIMARY KEY,
        type TEXT NOT NULL,
        content TEXT NOT NULL,
        context TEXT,
        category TEXT,
        timestamp DATETIME NOT NULL,
        priority INTEGER DEFAULT 0,
        metadata JSON
    );
    CREATE INDEX IF NOT EXISTS idx_type ON memories(type);
    CREATE INDEX IF NOT EXISTS idx_timestamp ON memories(timestamp);
    CREATE INDEX IF NOT EXISTS idx_priority ON memories(priority);
    CREATE VIRTUAL TABLE IF NOT EXISTS memories_fts USING FTS5(content, context);
    `
    
    _, err = db.Exec(schema)
    if err != nil {
        return nil, err
    }
    
    return &SQLiteMemoryStore{db: db}, nil
}

func (s *SQLiteMemoryStore) Save(entry *MemoryEntry) error {
    query := `
    INSERT INTO memories (id, type, content, context, category, timestamp, priority, metadata)
    VALUES (?, ?, ?, ?, ?, ?, ?, ?)
    `
    
    metadataJSON, _ := json.Marshal(entry.Metadata)
    
    _, err := s.db.Exec(query,
        entry.ID,
        entry.Type,
        entry.Content,
        entry.Context,
        entry.Category,
        entry.Timestamp,
        entry.Priority,
        metadataJSON,
    )
    
    return err
}

func (s *SQLiteMemoryStore) GetByTaskID(taskID string) []*MemoryEntry {
    query := `
    SELECT * FROM memories 
    WHERE metadata->>'task_id' = ?
    ORDER BY timestamp DESC
    `
    
    rows, err := s.db.Query(query, taskID)
    if err != nil {
        return nil
    }
    defer rows.Close()
    
    var entries []*MemoryEntry
    for rows.Next() {
        var entry MemoryEntry
        var metadataJSON string
        rows.Scan(&entry.ID, &entry.Type, &entry.Content, &entry.Context,
            &entry.Category, &entry.Timestamp, &entry.Priority, &metadataJSON)
        json.Unmarshal([]byte(metadataJSON), &entry.Metadata)
        entries = append(entries, &entry)
    }
    
    return entries
}
```

### 3.3 记忆检索器

```go
package memory

// MemoryRetriever 记忆检索器
type MemoryRetriever interface {
    Search(query string, limit int) []*MemoryEntry
    FindSimilar(content string) []*MemoryEntry
    GetRecent(limit int) []*MemoryEntry
    GetByCategory(category string) []*MemoryEntry
}

// VectorMemoryRetriever 向量检索器（支持语义相似度）
type VectorMemoryRetriever struct {
    store       MemoryStore
    embedder    TextEmbedder
    vectorStore VectorStore
}

func (r *VectorMemoryRetriever) Search(query string, limit int) []*MemoryEntry {
    // 1. 生成查询向量
    queryVector := r.embedder.Embed(query)
    
    // 2. 向量检索
    ids := r.vectorStore.Search(queryVector, limit)
    
    // 3. 从存储中获取条目
    entries := make([]*MemoryEntry, 0, len(ids))
    for _, id := range ids {
        entry, err := r.store.Get(id)
        if err == nil {
            entries = append(entries, entry)
        }
    }
    
    return entries
}

func (r *VectorMemoryRetriever) FindSimilar(content string) []*MemoryEntry {
    vector := r.embedder.Embed(content)
    ids := r.vectorStore.Search(vector, 5) // 找最相似的 5 条
    
    entries := make([]*MemoryEntry, 0, len(ids))
    for _, id := range ids {
        entry, err := r.store.Get(id)
        if err == nil && r.isSimilarEnough(content, entry.Content) {
            entries = append(entries, entry)
        }
    }
    
    return entries
}
```

## 4. Pipeline 编排器设计

基于 `internal/pipeline/orchestrator.go` 的状态机实现。

### 4.1 流水线状态定义

```go
package core

type PipelineState string

const (
    StatePending    PipelineState = "pending"
    StateParsing    PipelineState = "parsing"
    StateDeveloping PipelineState = "developing"
    StateTesting    PipelineState = "testing"
    StateChecking   PipelineState = "checking"
    StateRecovering PipelineState = "recovering"
    StateRollback   PipelineState = "rollback"
    StateCompleted  PipelineState = "completed"
    StateFailed     PipelineState = "failed"
)
```

### 4.2 编排器核心逻辑

Orchestrator 维护当前状态并负责将任务传递给注册的 Agent。

```go
package pipeline

// Orchestrator 驱动多 Agent 流水线
type Orchestrator struct {
    state    core.PipelineState
    config   *core.Config
    store    *memory.Store
    llm      llm.Provider
    bus      events.Bus
    registry *agents.Registry // 包含所有可用的 Agent
}

func (o *Orchestrator) Run(ctx context.Context, input *core.Input) (*core.Output, error) {
    o.transition(core.StateParsing)

    for {
        select {
        case <-ctx.Done():
            return nil, ctx.Err()
        default:
        }

        switch o.state {
        case core.StateParsing:
            out, err := o.runAgent(ctx, "agent-parser", input.TaskDescription, nil)
            if err != nil {
                o.transition(core.StateRecovering)
                continue
            }
            o.transition(core.StateDeveloping)

        case core.StateDeveloping:
            // 使用 Parser 的 Output 或当前 Task 作为输入
            out, err := o.runAgent(ctx, "agent-developer", input.TaskDescription, nil)
            if err != nil {
                o.transition(core.StateRecovering)
                continue
            }
            o.transition(core.StateTesting)

        case core.StateTesting:
            _, err := o.runAgent(ctx, "agent-tester", out.Message, nil)
            if err != nil {
                o.transition(core.StateRecovering)
                continue
            }
            o.transition(core.StateChecking)
            
        case core.StateRecovering:
            // Recovery Agent 尝试修复
            // 如果修复失败或超过重试次数，转到 Rollback
            o.transition(core.StateDeveloping) // 假定重入 Developing

        case core.StateCompleted:
            return &core.Output{Status: core.StatusSuccess, Message: "Done"}, nil
            
        case core.StateRollback:
            return nil, fmt.Errorf("pipeline failed after retries")
        }
    }
}

func (o *Orchestrator) runAgent(ctx context.Context, id, prompt string, history []core.Message) (*core.Output, error) {
    agent, _ := o.registry.Get(id)
    
    // 构建 AgentInput 并执行
    input := core.Input{ 
        TaskDescription: prompt, 
        History: history,
    }
    out, err := agent.Execute(ctx, input)
    if err != nil {
        o.bus.Publish(ctx, events.Event{Type: events.TypeAgentError, Agent: id})
        return nil, err
    }
    
    o.bus.Publish(ctx, events.Event{Type: events.TypeAgentComplete, Agent: id})
    return out, nil
}

// transition 修改状态并发布事件
func (o *Orchestrator) transition(s core.PipelineState) {
    o.state = s
    o.bus.Publish(ctx, events.Event{
        Type:    events.TypeStateChange,
        Payload: map[string]any{"state": string(s)},
    })
}
```

### 4.3 Event Bus 系统集成

Orchestrator 使用事件总线发布状态变化，支持外部订阅（如 CLI 进度条、日志记录）。

```go
package events

type Bus interface {
    Publish(ctx context.Context, event Event) error
    Subscribe(t Type, h Handler) error
}

// InMemoryBus 默认实现
type InMemoryBus struct {
    mu       sync.RWMutex
    handlers map[Type][]Handler
}
```

## 5. LLM Provider 与自定义 Agent

### 5.1 LLM Provider 接口

```go
package llm

type Provider interface {
    Name() string
    Chat(ctx context.Context, messages []core.Message, tools []core.Tool) (*core.Output, error)
}

// OpenAIProvider 实现（位于 internal/llm/openai.go）
type OpenAIProvider struct {
    BaseURL string
    APIKey  string
    Model   string
}
```

### 5.2 自定义 Agent 注册 (`internal/registry`)

支持从 YAML 文件加载动态 Agent 定义。

```go
package registry

// AgentDef 对应 YAML 结构 (agents/custom_analyst.yaml)
type AgentDef struct {
    Name        string    `yaml:"name"`
    Role        string    `yaml:"role"`
    Description string    `yaml:"description"`
    Prompt      PromptDef `yaml:"prompt"`
}

// Loader 处理 YAML 文件的扫描和注册
type Loader struct {
    registry *agents.Registry
}

func (l *Loader) LoadFromDir(dir string) error {
    // 1. 读取目录下的 *.yaml
    // 2. 解析为 AgentDef
    // 3. 根据 Role 和 Prompt 创建 core.CustomAgent
    // 4. 注册到 Registry
}
```

### 5.1 MCP 协议定义

```go
package mcp

// MCP Protocol 基于 JSON-RPC 2.0

// MCPRequest MCP 请求
type MCPRequest struct {
    JSONRPC string          `json:"jsonrpc"` // "2.0"
    ID      string          `json:"id"`
    Method  string          `json:"method"`
    Params  json.RawMessage `json:"params,omitempty"`
}

// MCPResponse MCP 响应
type MCPResponse struct {
    JSONRPC string          `json:"jsonrpc"` // "2.0"
    ID      string          `json:"id"`
    Result  json.RawMessage `json:"result,omitempty"`
    Error   *MCPError       `json:"error,omitempty"`
}

// MCPError MCP 错误
type MCPError struct {
    Code    int    `json:"code"`
    Message string `json:"message"`
    Data    any    `json:"data,omitempty"`
}

// MCP Tool 定义
type MCPTool struct {
    Name        string                 `json:"name"`
    Description string                 `json:"description"`
    InputSchema map[string]interface{} `json:"inputSchema"`
}

// MCPToolResult Tool 执行结果
type MCPToolResult struct {
    Content []MCPContent `json:"content"`
    IsError bool         `json:"isError,omitempty"`
}

// MCPContent 内容类型
type MCPContent struct {
    Type string `json:"type"` // "text", "image", "resource"
    Text string `json:"text,omitempty"`
    Data string `json:"data,omitempty"`
    MIMEType string `json:"mimeType,omitempty"`
    URI  string `json:"uri,omitempty"`
}
```

### 5.2 MCP Client 实现

```go
package mcp

// MCPClient MCP 客户端
type MCPClient struct {
    transport   MCPTransport
    tools       map[string]*MCPTool
    resources   map[string]*MCPResource
    prompts     map[string]*MCPPrompt
    initialized bool
}

// MCPTransport 传输层接口
type MCPTransport interface {
    Connect() error
    Disconnect() error
    Send(request *MCPRequest) (*MCPResponse, error)
    IsConnected() bool
}

// StdioTransport 标准输入输出传输
type StdioTransport struct {
    cmd    *exec.Cmd
    stdin  io.WriteCloser
    stdout io.Reader
}

func NewStdioTransport(command string, args ...string) *StdioTransport {
    cmd := exec.Command(command, args...)
    return &StdioTransport{cmd: cmd}
}

func (t *StdioTransport) Connect() error {
    t.stdin, _ = t.cmd.StdinPipe()
    t.stdout, _ = t.cmd.StdoutPipe()
    return t.cmd.Start()
}

func (t *StdioTransport) Send(request *MCPRequest) (*MCPResponse, error) {
    // 发送请求
    requestJSON, _ := json.Marshal(request)
    t.stdin.Write(requestJSON)
    t.stdin.Write([]byte("\n"))
    
    // 读取响应
    reader := bufio.NewReader(t.stdout)
    responseLine, _ := reader.ReadString('\n')
    
    var response MCPResponse
    json.Unmarshal([]byte(responseLine), &response)
    
    return &response, nil
}

// HTTPTransport HTTP 传输（用于远程 MCP Server）
type HTTPTransport struct {
    baseURL string
    client  *http.Client
}

func (t *HTTPTransport) Send(request *MCPRequest) (*MCPResponse, error) {
    requestJSON, _ := json.Marshal(request)
    
    resp, err := t.client.Post(t.baseURL, "application/json", bytes.NewReader(requestJSON))
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    
    var response MCPResponse
    json.NewDecoder(resp.Body).Decode(&response)
    
    return &response, nil
}
```

### 5.3 MCP Tools 调用

```go
package mcp

// Initialize 初始化 MCP 连接
func (c *MCPClient) Initialize() error {
    request := &MCPRequest{
        JSONRPC: "2.0",
        ID:      generateID(),
        Method:  "initialize",
        Params:  json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{}}`),
    }
    
    response, err := c.transport.Send(request)
    if err != nil {
        return err
    }
    
    c.initialized = true
    
    // 获取可用 Tools
    tools, err := c.ListTools()
    if err == nil {
        c.tools = tools
    }
    
    return nil
}

// ListTools 获取可用 Tools
func (c *MCPClient) ListTools() (map[string]*MCPTool, error) {
    request := &MCPRequest{
        JSONRPC: "2.0",
        ID:      generateID(),
        Method:  "tools/list",
    }
    
    response, err := c.transport.Send(request)
    if err != nil {
        return nil, err
    }
    
    var result struct {
        Tools []*MCPTool `json:"tools"`
    }
    json.Unmarshal(response.Result, &result)
    
    tools := make(map[string]*MCPTool)
    for _, tool := range result.Tools {
        tools[tool.Name] = tool
    }
    
    return tools, nil
}

// CallTool 调用 Tool
func (c *MCPClient) CallTool(name string, arguments map[string]interface{}) (*MCPToolResult, error) {
    params := map[string]interface{}{
        "name":      name,
        "arguments": arguments,
    }
    paramsJSON, _ := json.Marshal(params)
    
    request := &MCPRequest{
        JSONRPC: "2.0",
        ID:      generateID(),
        Method:  "tools/call",
        Params:  paramsJSON,
    }
    
    response, err := c.transport.Send(request)
    if err != nil {
        return nil, err
    }
    
    if response.Error != nil {
        return nil, fmt.Errorf("tool error: %s", response.Error.Message)
    }
    
    var result MCPToolResult
    json.Unmarshal(response.Result, &result)
    
    return &result, nil
}

// GetResource 获取 Resource
func (c *MCPClient) GetResource(uri string) ([]MCPContent, error) {
    params := map[string]interface{}{
        "uri": uri,
    }
    paramsJSON, _ := json.Marshal(params)
    
    request := &MCPRequest{
        JSONRPC: "2.0",
        ID:      generateID(),
        Method:  "resources/read",
        Params:  paramsJSON,
    }
    
    response, err := c.transport.Send(request)
    if err != nil {
        return nil, err
    }
    
    var result struct {
        Contents []MCPContent `json:"contents"`
    }
    json.Unmarshal(response.Result, &result)
    
    return result.Contents, nil
}
```

### 5.4 Agent 使用 MCP Tools

```go
package agent

// CoderAgent 代码生成 Agent（使用 MCP Tools）
type CoderAgent struct {
    BaseAgent
    mcpTools []string
}

func (a *CoderAgent) MCPTools() []string {
    return []string{
        "read_file",
        "write_file",
        "edit_file",
        "grep_search",
        "glob_search",
        "bash_execute",
        "git_status",
        "git_diff",
        "git_commit",
    }
}

func (a *CoderAgent) Execute(ctx context.Context, task *Task) (*Result, error) {
    // 1. 从记忆系统获取相关上下文
    memory := a.memory.Retrieve(task.Description, 10)
    
    // 2. 使用 MCP Tools 获取项目文件
    files, err := a.mcpClient.CallTool("glob_search", map[string]interface{}{
        "pattern": "**/*.go",
    })
    if err != nil {
        return nil, err
    }
    
    // 3. 分析现有代码结构
    structure := a.analyzeProjectStructure(files)
    
    // 4. 生成代码
    code := a.generateCode(task, structure, memory)
    
    // 5. 使用 MCP Tools 写入文件
    for _, file := range code.Files {
        _, err := a.mcpClient.CallTool("write_file", map[string]interface{}{
            "path":    file.Path,
            "content": file.Content,
        })
        if err != nil {
            return nil, err
        }
    }
    
    // 6. 使用 MCP Tools 执行 Git 操作
    _, err = a.mcpClient.CallTool("git_commit", map[string]interface{}{
        "message": "Generated code for " + task.Description,
    })
    if err != nil {
        return nil, err
    }
    
    return &Result{
        Success: true,
        Output:  code.Summary,
    }, nil
}
```

## 6. LLM Provider 多模型支持

### 6.1 LLM Provider 接口

```go
package llm

type Provider interface {
    Name() string
    Complete(ctx context.Context, messages []Message, opts *Options) (*Response, error)
    Stream(ctx context.Context, messages []Message, opts *Options) (<-chan StreamChunk, error)
    CountTokens(text string) int
    MaxTokens() int
    Capabilities() Capabilities
}

type Message struct {
    Role    string `json:"role"`    // "system", "user", "assistant"
    Content string `json:"content"`
}

type Options struct {
    Model       string  `json:"model"`
    Temperature float64 `json:"temperature"`
    MaxTokens   int     `json:"max_tokens"`
    TopP        float64 `json:"top_p,omitempty"`
    Stop        []string `json:"stop,omitempty"`
}

type Response struct {
    ID      string   `json:"id"`
    Content string   `json:"content"`
    Model   string   `json:"model"`
    Usage   Usage    `json:"usage"`
}

type Usage struct {
    PromptTokens     int `json:"prompt_tokens"`
    CompletionTokens int `json:"completion_tokens"`
    TotalTokens      int `json:"total_tokens"`
}

type StreamChunk struct {
    Content string
    Done    bool
}

type Capabilities struct {
    MaxTokens   int
    ContextWindow int
    Streaming   bool
    Vision      bool
    FunctionCall bool
}
```

### 6.2 OpenAI Provider

```go
package llm

type OpenAIProvider struct {
    client  *openai.Client
    model   string
    baseURL string
    apiKey  string
}

func NewOpenAIProvider(cfg LLMConfig) *OpenAIProvider {
    config := openai.DefaultConfig(cfg.APIKey)
    if cfg.BaseURL != "" {
        config.BaseURL = cfg.BaseURL
    }
    return &OpenAIProvider{
        client:  openai.NewClientWithConfig(config),
        model:   cfg.Model,
        baseURL: cfg.BaseURL,
        apiKey:  cfg.APIKey,
    }
}

func (p *OpenAIProvider) Complete(ctx context.Context, messages []Message, opts *Options) (*Response, error) {
    if opts == nil {
        opts = &Options{Model: p.model}
    }
    if opts.Model == "" {
        opts.Model = p.model
    }

    openaiMsgs := make([]openai.ChatCompletionMessage, len(messages))
    for i, m := range messages {
        openaiMsgs[i] = openai.ChatCompletionMessage{Role: m.Role, Content: m.Content}
    }

    resp, err := p.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
        Model:       opts.Model,
        Messages:    openaiMsgs,
        Temperature: opts.Temperature,
        MaxTokens:   opts.MaxTokens,
    })
    if err != nil {
        return nil, err
    }

    return &Response{
        ID:      resp.ID,
        Content: resp.Choices[0].Message.Content,
        Model:   resp.Model,
        Usage: Usage{
            PromptTokens:     resp.Usage.PromptTokens,
            CompletionTokens: resp.Usage.CompletionTokens,
            TotalTokens:      resp.Usage.TotalTokens,
        },
    }, nil
}

func (p *OpenAIProvider) Name() string       { return "openai" }
func (p *OpenAIProvider) MaxTokens() int     { return 128000 }
func (p *OpenAIProvider) Capabilities() Capabilities {
    return Capabilities{
        MaxTokens: 128000, ContextWindow: 128000,
        Streaming: true, Vision: true, FunctionCall: true,
    }
}
```

### 6.3 Claude Provider

```go
package llm

type ClaudeProvider struct {
    client  *anthropic.Client
    model   string
    baseURL string
    apiKey  string
}

func NewClaudeProvider(cfg LLMConfig) *ClaudeProvider {
    return &ClaudeProvider{
        client:  anthropic.NewClient(cfg.APIKey),
        model:   cfg.Model,
        apiKey:  cfg.APIKey,
    }
}

func (p *ClaudeProvider) Complete(ctx context.Context, messages []Message, opts *Options) (*Response, error) {
    if opts == nil { opts = &Options{Model: p.model} }
    if opts.Model == "" { opts.Model = p.model }

    resp, err := p.client.Messages.Create(ctx, anthropic.MessageRequest{
        Model:     opts.Model,
        MaxTokens: opts.MaxTokens,
        Messages:  toAnthropicMessages(messages),
        System:    extractSystemPrompt(messages),
    })
    if err != nil { return nil, err }

    return &Response{
        ID: resp.ID, Content: resp.Content[0].Text,
        Model: resp.Model,
        Usage: Usage{
            PromptTokens: resp.Usage.InputTokens,
            CompletionTokens: resp.Usage.OutputTokens,
            TotalTokens: resp.Usage.InputTokens + resp.Usage.OutputTokens,
        },
    }, nil
}

func (p *ClaudeProvider) Name() string { return "claude" }
func (p *ClaudeProvider) MaxTokens() int { return 200000 }
```

### 6.4 本地模型 Provider (Ollama)

```go
package llm

type LocalProvider struct {
    baseURL string
    model   string
    client  *http.Client
}

func NewLocalProvider(cfg LLMConfig) *LocalProvider {
    return &LocalProvider{
        baseURL: cfg.BaseURL,
        model:   cfg.Model,
        client:  &http.Client{Timeout: 5 * time.Minute},
    }
}

func (p *LocalProvider) Complete(ctx context.Context, messages []Message, opts *Options) (*Response, error) {
    if opts == nil { opts = &Options{Model: p.model} }
    if opts.Model == "" { opts.Model = p.model }

    reqBody := map[string]any{
        "model":    opts.Model,
        "messages": messages,
        "stream":   false,
        "options": map[string]any{
            "temperature": opts.Temperature,
        },
    }

    resp, err := p.client.Post(p.baseURL+"/api/chat", "application/json",
        bytes.NewBuffer(mustMarshal(reqBody)))
    // ... parse response
}

func (p *LocalProvider) Name() string { return "local" }
```

### 6.5 Provider Factory

```go
package llm

type LLMConfig struct {
    Provider string `yaml:"provider"` // "openai", "claude", "local"
    Model    string `yaml:"model"`
    APIKey   string `yaml:"api_key"`
    BaseURL  string `yaml:"base_url"`
}

func NewProvider(cfg LLMConfig) Provider {
    switch cfg.Provider {
    case "openai", "azure":
        return NewOpenAIProvider(cfg)
    case "claude", "anthropic":
        return NewClaudeProvider(cfg)
    case "local", "ollama":
        return NewLocalProvider(cfg)
    default:
        return NewOpenAIProvider(cfg)
    }
}
```

### 6.6 Agent 指定模型机制

```go
package agent

// Agent 可指定自己的模型，不指定则使用全局默认
type AgentModelConfig struct {
    Model       string  `yaml:"model"`       // 模型名称
    Temperature float64 `yaml:"temperature"` // 温度（覆盖全局）
    MaxTokens   int     `yaml:"max_tokens"`  // 最大 Token 数
}

// BaseAgent 携带可选的模型配置
type BaseAgent struct {
    type_        AgentType
    name         string
    memory       *MemorySystem
    session      *SessionStore
    mcpClient    *MCPClient
    llmProvider  LLMProvider       // 默认 Provider
    modelConfig  *AgentModelConfig // Agent 专属模型配置（可选）
}

// ExecuteWithModel 如果 Agent 指定了模型，使用该模型
func (a *BaseAgent) complete(ctx context.Context, messages []Message) (*Response, error) {
    opts := &Options{
        Temperature: 0.7,
        MaxTokens:   4096,
    }

    // Agent 指定的模型覆盖全局默认
    if a.modelConfig != nil {
        opts.Model = a.modelConfig.Model
        opts.Temperature = a.modelConfig.Temperature
        opts.MaxTokens = a.modelConfig.MaxTokens
    }

    return a.llmProvider.Complete(ctx, messages, opts)
}
```

### 6.7 多模型配置示例

```yaml
# autodev.yaml
llm:
  # 全局默认模型 (供所有 Agent 共用)
  provider: openai
  model: gpt-4o
  api_key: ${OPENAI_API_KEY}

  # 模型池 (Agent 可选择使用)
  models:
    - name: openai/gpt-4o
      provider: openai
      model: gpt-4o
      api_key: ${OPENAI_API_KEY}
    - name: claude/sonnet-3.5
      provider: claude
      model: claude-sonnet-4-20250514
      api_key: ${ANTHROPIC_API_KEY}
    - name: local/qwen2.5
      provider: local
      model: qwen2.5:72b
      base_url: http://localhost:11434
    - name: openai/o1-mini
      provider: openai
      model: o1-mini
      api_key: ${OPENAI_API_KEY}

# Agent 各自指定模型
agents:
  parser:
    enabled: true
    model: "openai/gpt-4o"         # 使用模型池中 gpt-4o
  coder:
    enabled: true
    model: "claude/sonnet-3.5"     # 使用 Claude Sonnet
  tester:
    enabled: true
    model: "openai/gpt-4o"
  checker:
    enabled: true
    model: "openai/o1-mini"        # o1-mini 适合审查推理
  recovery:
    enabled: true
    model: "claude/sonnet-3.5"

# 自定义 Agent 也可以指定模型
custom_agents:
  enabled: true
  directories:
    - ~/.autodev/agents
```

## 7. 状态机实现

### 7.1 任务状态机

```go
package statemachine

// TaskState 任务状态
type TaskState string

const (
    StateIdle        TaskState = "idle"
    StateParsing     TaskState = "parsing"
    StateAnalyzed    TaskState = "analyzed"
    StateDeveloping  TaskState = "developing"
    StateTesting     TaskState = "testing"
    StateChecking    TaskState = "checking"
    StateRecovering  TaskState = "recovering"
    StateRollback    TaskState = "rollback"
    StateCompleted   TaskState = "completed"
    StatePRDone      TaskState = "pr_done"
    StateFailed      TaskState = "failed"
)

// StateMachine 状态机
type StateMachine struct {
    currentState TaskState
    transitions  map[TaskState][]Transition
    handlers     map[TaskState]StateHandler
    session      *SessionStore
}

// Transition 状态转换
type Transition struct {
    From    TaskState
    To      TaskState
    Trigger string
    Condition func() bool
}

// StateHandler 状态处理器
type StateHandler func(ctx context.Context, task *Task) (*Result, error)

func NewStateMachine() *StateMachine {
    sm := &StateMachine{
        currentState: StateIdle,
        transitions:  make(map[TaskState][]Transition),
        handlers:     make(map[TaskState]StateHandler),
    }
    
    sm.setupTransitions()
    return sm
}

func (sm *StateMachine) setupTransitions() {
    // 定义状态转换规则
    sm.transitions[StateIdle] = []Transition{
        {To: StateParsing, Trigger: "start"},
    }
    
    sm.transitions[StateParsing] = []Transition{
        {To: StateAnalyzed, Trigger: "parse_success"},
        {To: StateFailed, Trigger: "parse_failed"},
    }
    
    sm.transitions[StateAnalyzed] = []Transition{
        {To: StateDeveloping, Trigger: "plan_ready"},
    }
    
    sm.transitions[StateDeveloping] = []Transition{
        {To: StateTesting, Trigger: "code_done"},
        {To: StateRecovering, Trigger: "code_failed"},
    }
    
    sm.transitions[StateTesting] = []Transition{
        {To: StateChecking, Trigger: "test_done"},
        {To: StateRecovering, Trigger: "test_failed"},
    }
    
    sm.transitions[StateChecking] = []Transition{
        {To: StateCompleted, Trigger: "check_passed"},
        {To: StateRecovering, Trigger: "check_failed"},
    }
    
    sm.transitions[StateRecovering] = []Transition{
        {To: StateTesting, Trigger: "fix_success"},
        {To: StateRollback, Trigger: "fix_failed"},
    }
    
    sm.transitions[StateRollback] = []Transition{
        {To: StateDeveloping, Trigger: "retry"},
        {To: StateFailed, Trigger: "max_retries"},
    }
    
    sm.transitions[StateCompleted] = []Transition{
        {To: StatePRDone, Trigger: "create_pr"},
    }
}

func (sm *StateMachine) Transition(trigger string) error {
    transitions := sm.transitions[sm.currentState]
    
    for _, t := range transitions {
        if t.Trigger == trigger {
            if t.Condition != nil && !t.Condition() {
                return fmt.Errorf("condition not met for transition")
            }
            
            sm.currentState = t.To
            return nil
        }
    }
    
    return fmt.Errorf("invalid transition: %s from %s", trigger, sm.currentState)
}

func (sm *StateMachine) Execute(ctx context.Context, task *Task) error {
    for sm.currentState != StateCompleted && sm.currentState != StateFailed {
        handler := sm.handlers[sm.currentState]
        if handler == nil {
            return fmt.Errorf("no handler for state %s", sm.currentState)
        }
        
        result, err := handler(ctx, task)
        if err != nil {
            sm.Transition(sm.getFailureTrigger())
            continue
        }
        
        sm.Transition(sm.getSuccessTrigger())
    }
    
    return nil
}
```

## 7. CLI 与配置实现

### 7.1 Cobra 入口 (`cmd/autodev/main.go`)

```go
package main

func main() {
	var provider, model, apiKey, agentsDir string
	
	var runCmd = &cobra.Command{
		Use:   "run [task]",
		Short: "Execute a development task",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPipeline(args[0], provider, model, apiKey, agentsDir)
		},
	}

	runCmd.Flags().StringVar(&provider, "provider", "openai", "LLM Provider (openai)")
	runCmd.Flags().StringVar(&model, "model", "gpt-4o", "LLM Model")
	runCmd.Flags().StringVar(&apiKey, "api-key", "", "API Key (or OPENAI_API_KEY env)")
	runCmd.Flags().StringVar(&agentsDir, "agents-dir", "./agents", "Custom agents YAML dir")

	rootCmd.AddCommand(runCmd)
	_ = rootCmd.Execute()
}

func runPipeline(task, provider, model, apiKey, agentsDir string) error {
    // ...
}
```

### 7.3 状态机验证 (`internal/llm/mock.go`)

实现了一个带有故障注入能力的 Mock LLM Provider，专门用于验证 Pipeline 的状态机流转。
- **特性**: 支持 `FailCount` 配置，模拟第 N 次调用失败。
- **流程验证**: Parser 失败后触发 Recovery Agent，恢复成功后自动回到 Developing 阶段，最终进入 Completed。


### 7.2 全局配置 (`internal/core/config.go`)

```go
package core

type Config struct {
	WorkDir   string
	DataDir   string
	MemoryDB  string
	Provider  string
	Model     string
}
```
  db_path: ~/.autodev/session.db
  auto_save: true
  checkpoint_interval: 60s

mcp:
  enabled: true
  servers:
    - name: filesystem
      command: mcp-server-filesystem
      args: ["--root", "."]
    - name: git
      command: mcp-server-git
    - name: shell
      command: mcp-server-shell

coverage:
  enabled: true
  threshold:
    lines: 80
    branches: 70
    functions: 80

lint:
  enabled: true
  fix: false
  config: .eslintrc.js

git:
  auto_commit: true
  auto_pr: true
  branch_prefix: autodev/

output:
  format: console
  log_level: info
  log_file: ~/.autodev/autodev.log
```

### 8.2 Viper 配置加载

```go
package config

type Config struct {
    Version  string
    LLM      LLMConfig
    Agents   AgentsConfig
    Memory   MemoryConfig
    Session  SessionConfig
    MCP      MCPConfig
    Coverage CoverageConfig
    Lint     LintConfig
    Git      GitConfig
    Output   OutputConfig
}

func LoadConfig() (*Config, error) {
    viper.SetConfigName("autodev")
    viper.SetConfigType("yaml")
    viper.AddConfigPath(".")
    viper.AddConfigPath("$HOME/.autodev")
    
    if err := viper.ReadInConfig(); err != nil {
        return nil, err
    }
    
    var config Config
    if err := viper.Unmarshal(&config); err != nil {
        return nil, err
    }
    
    return &config, nil
}
```

## 9. 并发控制

### 9.1 多 Agent 并发执行

```go
package orchestrator

// ParallelAgentExecution 并发执行多个 Agent
func (o *AgentOrchestrator) ParallelAgentExecution(
    ctx context.Context,
    tasks []*Task,
) ([]*Result, error) {
    g, ctx := errgroup.WithContext(ctx)
    
    results := make([]*Result, len(tasks))
    
    for i, task := range tasks {
        g.Go(func() error {
            agent := o.router.Route(task)
            result, err := agent.Execute(ctx, task)
            if err != nil {
                return err
            }
            results[i] = result
            return nil
        })
    }
    
    if err := g.Wait(); err != nil {
        return nil, err
    }
    
    return results, nil
}
```

## 10. 项目结构

```
autodev-agent/
├── cmd/
│   ├── root.go          # 根命令
│   ├── run.go           # run 命令
│   ├── config.go        # config 命令
│   ├── status.go        # status 命令
│   ├── history.go       # history 命令
│   ├── rollback.go      # rollback 命令
│   └── interactive.go   # interactive 命令
├── internal/
│   ├── agent/
│   │   ├── base.go          # BaseAgent
│   │   ├── parser.go        # ParserAgent
│   │   ├── coder.go         # CoderAgent
│   │   ├── tester.go        # TesterAgent
│   │   ├── checker.go       # CheckerAgent
│   │   ├── recovery.go      # RecoveryAgent
│   │   └── interface.go     # Agent 接口
│   ├── orchestrator/
│   │   ├── orchestrator.go  # Agent 编排器
│   │   ├── router.go        # Agent 路由器
│   │   └── coordinator.go   # Agent 协调器
│   ├── memory/
│   │   ├── system.go        # 记忆系统
│   │   ├── store.go         # 记忆存储
│   │   ├── retriever.go     # 记忆检索
│   │   └── compressor.go    # 记忆压缩
│   ├── session/
│   │   ├── session.go       # 会话定义
│   │   ├── store.go         # 会话存储
│   │   ├── snapshot.go      # 快照管理
│   │   └── resume.go        # 断点续传
│   ├── mcp/
│   │   ├── protocol.go      # MCP 协议
│   │   ├── client.go        # MCP 客户端
│   │   ├── transport.go     # MCP 传输层
│   │   └── tools.go         # MCP Tools
│   ├── statemachine/
│   │   ├── machine.go       # 状态机
│   │   ├── transitions.go   # 状态转换
│   │   └── handlers.go      # 状态处理器
│   ├── llm/
│   │   ├── provider.go      # LLM Provider
│   │   ├── openai.go        # OpenAI 实现
│   │   ├── claude.go        # Claude 实现
│   │   ├── local.go         # 本地模型实现
│   │   └── token.go         # Token 管理
│   ├── context/
│   │   ├── manager.go       # 上下文管理器
│   │   ├── cache.go         # 文件缓存
│   │   └── project.go       # 项目分析
│   ├── git/
│   │   ├── operations.go    # Git 操作
│   │   ├── branch.go        # 分支管理
│   │   ├── commit.go        # 提交管理
│   │   └── pr.go            # PR 创建
│   ├── validator/
│   │   ├── checker.go       # 验收检查
│   │   ├── coverage.go      # 覆盖率检查
│   │   ├── lint.go          # 静态分析
│   │   └── quality.go       # 质量门禁
│   └── task/
│   │   ├── task.go          # 任务定义
│   │   ├── plan.go          # 任务计划
│   │   └── result.go        # 任务结果
│   └── config/
│   │   ├── config.go        # 配置加载
│   │   └── schema.go        # 配置结构
├── pkg/
│   ├── utils/
│   │   ├── logger.go        # 日志工具
│   │   ├── uuid.go          # ID 生成
│   │   └── json.go          # JSON 工具
│   └── prompt/
│   │   ├── templates.go     # Prompt 模板
│   │   └── builder.go       # Prompt 构建器
├── configs/
│   └── autodev.yaml         # 默认配置
├── go.mod
├── go.sum
├── Makefile
└── README.md
```

## 11. Makefile

```makefile
.PHONY: build run test install clean

BINARY_NAME=autodev
VERSION=1.0.0
BUILD_DIR=./bin

build:
	go build -ldflags "-X main.Version=$(VERSION)" -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd

install:
	go install ./cmd

run:
	go run ./cmd run "$(TASK)"

test:
	go test -v ./...

clean:
	rm -rf $(BUILD_DIR)
	rm -rf ~/.autodev/*.db

lint:
	golangci-lint run ./...

fmt:
	go fmt ./...

deps:
	go mod download
	go mod tidy

docker:
	docker build -t autodev-agent:$(VERSION) .

.PHONY: all
all: deps fmt lint test build
```

## 12. 依赖关系图

```
┌─────────────┐
│   CLI App   │
└──────┬──────┘
       │
       ├──► Orchestrator
       │         │
       │         ├──► Agents (Parser, Coder, Tester, Checker, Recovery)
       │         │         │
       │         │         ├──► Memory System
       │         │         ├──► Session Store
       │         │         ├──► MCP Client
       │         │         └──► LLM Provider
       │         │
       │         ├──► State Machine
       │         └──► Router
       │
       ├──► Config (Viper)
       ├──► Logger (Logrus/Zap)
       └──► Git Operations (go-git)

Data Stores:
┌──────────────┐     ┌──────────────┐
│ Memory Store │     │Session Store │
│  (SQLite)    │     │  (SQLite)    │
└──────┬───────┘     └──────┬───────┘
       │                    │
       └──► FTS5 Search     └──► Snapshots

External Services:
┌──────────────┐     ┌──────────────┐
│ LLM Provider │     │ MCP Servers  │
│  (OpenAI/    │     │  (Stdio/     │
│   Claude)    │     │   HTTP)      │
└──────────────┘     └──────────────┘
```

## 13. 关键技术点

### 13.1 Go 并发模式

- **errgroup**：多 Agent 并发执行，统一错误处理
- **channels**：Agent 间消息传递
- **context**：超时控制、取消信号
- **sync.Mutex**：共享资源保护

### 13.2 SQLite 优化

- **FTS5 全文搜索**：记忆内容语义检索
- **JSON 存储**：复杂对象持久化
- **索引优化**：高频查询字段索引
- **事务处理**：批量操作原子性

### 13.3 MCP 协议实现

- **JSON-RPC 2.0**：标准 RPC 协议
- **Stdio 传输**：本地 MCP Server 通信
- **HTTP 传输**：远程 MCP Server 通信
- **异步响应**：流式 Tool 结果处理

### 13.4 记忆系统优化

- **向量检索**：语义相似度搜索（可选）
- **记忆压缩**：长文本智能摘要
- **优先级排序**：重要性加权
- **去重合并**：避免冗余记忆

## 14. 人工确认模块设计

### 14.1 确认点定义

```go
package confirmation

import "time"

type ConfirmationLevel string

const (
	LevelCritical ConfirmationLevel = "critical"
	LevelWarning  ConfirmationLevel = "warning"
	LevelInfo     ConfirmationLevel = "info"
)

type ConfirmationPoint struct {
	ID      string               `yaml:"id"`
	Name    string               `yaml:"name"`
	Trigger string               `yaml:"trigger"`
	Message string               `yaml:"message"`
	Options []ConfirmationOption `yaml:"options"`
	Level   ConfirmationLevel    `yaml:"level"`
	Required bool                `yaml:"required"`
	Timeout time.Duration        `yaml:"timeout"`
}

type ConfirmationOption struct {
	Value       string `yaml:"value"`
	Label       string `yaml:"label"`
	Description string `yaml:"description"`
}

type ConfirmationResult struct {
	PointID   string    `yaml:"point_id"`
	Choice    string    `yaml:"choice"`
	Timestamp time.Time `yaml:"timestamp"`
	Comments  string    `yaml:"comments"`
}
```

### 14.2 确认处理器

```go
package confirmation

import (
	"fmt"
	"os"
	"bufio"
	"strings"
	"errors"
)

var (
	ErrConfirmationTimeout = errors.New("confirmation timeout")
	ErrConfirmationDenied  = errors.New("confirmation denied")
)

type ConfirmationHandler interface {
	Request(point ConfirmationPoint) (ConfirmationResult, error)
}

type CLIConfirmationHandler struct {
	Interactive bool
	AutoApprove bool
}

func (h *CLIConfirmationHandler) Request(point ConfirmationPoint) (ConfirmationResult, error) {
	if h.AutoApprove && point.Level != LevelCritical {
		return ConfirmationResult{
			PointID: point.ID,
			Choice:  point.Options[0].Value,
		}, nil
	}
	if !h.Interactive {
		return ConfirmationResult{}, ErrConfirmationDenied
	}

	fmt.Printf("\n[%s] %s\n", strings.ToUpper(string(point.Level)), point.Message)
	for i, opt := range point.Options {
		fmt.Printf("  %d. %s - %s\n", i+1, opt.Label, opt.Description)
	}

	reader := bufio.NewReader(os.Stdin)
	done := make(chan ConfirmationResult, 1)
	go func() {
		fmt.Print("\n请选择 (数字): ")
		input, _ := reader.ReadString('\n')
		choiceNum := strings.TrimSpace(input)
		for i, opt := range point.Options {
			if fmt.Sprintf("%d", i+1) == choiceNum {
				done <- ConfirmationResult{PointID: point.ID, Choice: opt.Value}
				return
			}
		}
		done <- ConfirmationResult{PointID: point.ID, Choice: "invalid"}
	}()

	select {
	case result := <-done:
		return result, nil
	case <-time.After(point.Timeout):
		return ConfirmationResult{}, ErrConfirmationTimeout
	}
}
```

### 14.3 内置确认点

```go
package confirmation

import "time"

var PredefinedPoints = map[string]ConfirmationPoint{
	"pr_create": {
		ID: "pr_create", Name: "创建 Pull Request", Trigger: "before_pr",
		Message: "即将创建 Pull Request，请确认信息是否正确",
		Options: []ConfirmationOption{
			{Value: "approve", Label: "确认创建", Description: "PR 信息正确，继续创建"},
			{Value: "edit", Label: "编辑 PR 信息", Description: "修改 PR 标题或描述"},
			{Value: "reject", Label: "取消创建", Description: "不创建 PR，保留本地代码"},
		},
		Level: LevelWarning, Required: true, Timeout: 60 * time.Second,
	},
	"critical_issue": {
		ID: "critical_issue", Name: "Critical 级别问题", Trigger: "critical_lint_error",
		Message: "发现 Critical 级别问题，建议停止并人工审查",
		Options: []ConfirmationOption{
			{Value: "stop", Label: "停止任务", Description: "停止当前任务，保留快照"},
			{Value: "continue", Label: "继续执行", Description: "忽略警告继续"},
			{Value: "review", Label: "人工审查", Description: "等待人工审查后继续"},
		},
		Level: LevelCritical, Required: true, Timeout: 120 * time.Second,
	},
	"sensitive_operation": {
		ID: "sensitive_operation", Name: "敏感操作", Trigger: "delete_file_or_directory",
		Message: "即将执行敏感操作（删除文件/目录），请确认",
		Options: []ConfirmationOption{
			{Value: "approve", Label: "确认执行", Description: "操作安全，继续执行"},
			{Value: "reject", Label: "取消操作", Description: "不执行此操作"},
		},
		Level: LevelCritical, Required: true, Timeout: 60 * time.Second,
	},
	"force_fix": {
		ID: "force_fix", Name: "强制修复确认", Trigger: "after_max_retries",
		Message: "已达到最大重试次数 (3次)，是否尝试强制修复？",
		Options: []ConfirmationOption{
			{Value: "force_fix", Label: "尝试强制修复", Description: "使用最后手段修复"},
			{Value: "rollback", Label: "回退代码", Description: "回退到之前的稳定版本"},
			{Value: "stop", Label: "停止任务", Description: "停止任务并保存进度"},
		},
		Level: LevelWarning, Required: true, Timeout: 120 * time.Second,
	},
}
```

## 15. 进度反馈模块设计

### 15.1 进度事件定义

```go
package progress

import "time"

type ProgressEvent struct {
	TaskID    string         `json:"task_id"`
	SessionID string         `json:"session_id"`
	State     string         `json:"state"`
	StepIndex int            `json:"step_index"`
	StepName  string         `json:"step_name"`
	Detail    string         `json:"detail"`
	AgentType string         `json:"agent_type"`
	Timestamp time.Time      `json:"timestamp"`
	Meta      map[string]any `json:"meta,omitempty"`
}

type ProgressSummary struct {
	TaskID      string        `json:"task_id"`
	TotalSteps  int           `json:"total_steps"`
	CurrentStep int           `json:"current_step"`
	State       string        `json:"state"`
	Percentage  float64       `json:"percentage"`
	StartAt     time.Time     `json:"start_at"`
	Duration    time.Duration `json:"duration"`
	ETA         time.Time     `json:"eta,omitempty"`
}
```

### 15.2 进度报告器

```go
package progress

import (
	"fmt"
	"time"
	"github.com/cheggaaa/pb/v3"
)

type Reporter interface {
	Start(taskID string, totalSteps int)
	Update(event ProgressEvent)
	Complete(summary ProgressSummary)
	Fail(err error)
}

type TerminalReporter struct {
	bar        *pb.ProgressBar
	totalSteps int
	events     []ProgressEvent
	startTime  time.Time
}

func NewTerminalReporter() *TerminalReporter {
	return &TerminalReporter{events: make([]ProgressEvent, 0)}
}

func (r *TerminalReporter) Start(taskID string, totalSteps int) {
	r.totalSteps = totalSteps
	r.startTime = time.Now()
	fmt.Printf("\n任务: %s\n开始时间: %s\n\n", taskID, r.startTime.Format("2006-01-02 15:04:05"))

	template := `{{string . "state" | green}} {{percent . }} {{bar . "[" "=> " "-" "]" | blue}} {{count . }}/{{total . }} {{rtime .}}`
	r.bar = pb.New(totalSteps)
	r.bar.SetTemplateString(template)
	r.bar.Set("state", "准备中")
	r.bar.Start()
}

func (r *TerminalReporter) Update(event ProgressEvent) {
	r.events = append(r.events, event)
	if r.bar != nil {
		r.bar.Increment()
		r.bar.Set("state", getStateLabel(event.State))
	}
}

func (r *TerminalReporter) Complete(summary ProgressSummary) {
	if r.bar != nil { r.bar.Finish() }
	duration := time.Since(r.startTime)
	fmt.Printf("\n✓ 任务完成\n  耗时: %dm%ds\n  步骤: %d/%d\n  状态: %s\n",
		int(duration.Minutes()), int(duration.Seconds())%60,
		summary.CurrentStep, summary.TotalSteps, summary.State)
}

func (r *TerminalReporter) Fail(err error) {
	if r.bar != nil { r.bar.Abort(err) }
	fmt.Printf("\n✗ 任务失败\n  错误: %s\n", err.Error())
}

func getStateLabel(state string) string {
	labels := map[string]string{
		"idle": "准备中", "parsing": "解析任务", "developing": "开发代码",
		"testing": "执行测试", "checking": "验收检查", "recovering": "自动修复",
		"completed": "完成", "failed": "失败",
	}
	if l, ok := labels[state]; ok { return l }
	return state
}
```

## 16. 审计日志模块设计

### 16.1 日志条目

```go
package audit

import "time"

type LogLevel string
const (
	LogLevelInfo    LogLevel = "info"
	LogLevelWarning = "warning"
	LogLevelError   = "error"
)

type AuditEntry struct {
	ID         string    `json:"id"`
	Timestamp  time.Time `json:"timestamp"`
	TaskID     string    `json:"task_id"`
	AgentType  string    `json:"agent_type"`
	Operation  string    `json:"operation"`
	Target     string    `json:"target"`
	Result     string    `json:"result"`
	Error      string    `json:"error,omitempty"`
	Duration   time.Duration `json:"duration"`
	Parameters map[string]any `json:"parameters"`
	Level      LogLevel  `json:"level"`
}
```

### 16.2 审计日志器

```go
package audit

import (
	"database/sql"
	"encoding/json"
	_ "github.com/mattn/go-sqlite3"
)

type Logger interface {
	Log(entry *AuditEntry) error
	QueryByTask(taskID string) ([]*AuditEntry, error)
	ExportJSON() ([]byte, error)
}

type SQLiteLogger struct{ db *sql.DB }

func NewSQLiteLogger(dbPath string) (*SQLiteLogger, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil { return nil, err }
	_, _ = db.Exec(`CREATE TABLE IF NOT EXISTS audit_log (
		id TEXT PRIMARY KEY, timestamp DATETIME, task_id TEXT, agent_type TEXT,
		operation TEXT, target TEXT, result TEXT, error TEXT,
		duration_ms INTEGER, level TEXT DEFAULT 'info', parameters JSON)`)
	return &SQLiteLogger{db}, nil
}

func (l *SQLiteLogger) Log(e *AuditEntry) error {
	pj, _ := json.Marshal(e.Parameters)
	_, err := l.db.Exec(`INSERT INTO audit_log (id, timestamp, task_id, agent_type, operation, target, result, error, duration_ms, level, parameters)
		VALUES (?,?,?,?,?,?,?,?,?,?,?)`,
		e.ID, e.Timestamp, e.TaskID, e.AgentType, e.Operation, e.Target, e.Result, e.Error,
		int64(e.Duration/time.Millisecond), e.Level, pj)
	return err
}

func (l *SQLiteLogger) QueryByTask(taskID string) ([]*AuditEntry, error) {
	rows, err := l.db.Query(`SELECT * FROM audit_log WHERE task_id=? ORDER BY timestamp`, taskID)
	if err != nil { return nil, err }
	defer rows.Close()
	var entries []*AuditEntry
	for rows.Next() {
		e := &AuditEntry{}
		var durMs int64; var pj []byte
		rows.Scan(&e.ID, &e.Timestamp, &e.TaskID, &e.AgentType, &e.Operation, &e.Target, &e.Result, &e.Error, &durMs, &e.Level, &pj)
		e.Duration = time.Duration(durMs) * time.Millisecond
		json.Unmarshal(pj, &e.Parameters)
		entries = append(entries, e)
	}
	return entries, nil
}

func (l *SQLiteLogger) ExportJSON() ([]byte, error) {
	entries, _ := l.QueryByTask("")
	return json.MarshalIndent(entries, "", "  ")
}
```

### 16.3 关键审计点

```go
package audit

var AuditOperations = []string{
	"file_read", "file_write", "file_delete",
	"git_commit", "git_push", "git_reset",
	"pr_create", "mcp_tool_call", "llm_request",
	"session_create", "session_rollback", "snapshot_create",
	"test_execute", "lint_execute", "fix_apply",
}
```

## 17. 密钥管理模块设计

### 17.1 密钥管理器

```go
package secret

import (
	"crypto/aes"; "crypto/cipher"; "crypto/rand"
	"encoding/base64"; "io"; "os"; "path/filepath"
)

type Manager interface {
	Store(key, value string) error
	Retrieve(key string) (string, error)
	Delete(key string) error
}

type EncryptedManager struct{ store Storage; key []byte }

func NewEncryptedManager(baseDir string) (*EncryptedManager, error) {
	keyPath := filepath.Join(baseDir, "master.key")
	key, err := loadOrCreateKey(keyPath)
	if err != nil { return nil, err }
	store, _ := NewFileStorage(filepath.Join(baseDir, "secrets.db"))
	return &EncryptedManager{store, key}, nil
}

func (m *EncryptedManager) Store(key, value string) error {
	enc, err := encryptAESGCM(value, m.key)
	if err != nil { return err }
	return m.store.Save(key, enc)
}

func (m *EncryptedManager) Retrieve(key string) (string, error) {
	enc, err := m.store.Load(key)
	if err != nil { return "", err }
	return decryptAESGCM(enc, m.key)
}
```

### 17.2 AES-GCM 加密

```go
package secret

func encryptAESGCM(plaintext string, key []byte) (string, error) {
	block, _ := aes.NewCipher(key)
	gcm, _ := cipher.NewGCM(block)
	nonce := make([]byte, gcm.NonceSize())
	io.ReadFull(rand.Reader, nonce)
	ct := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ct), nil
}

func decryptAESGCM(ciphertext string, key []byte) (string, error) {
	data, _ := base64.StdEncoding.DecodeString(ciphertext)
	block, _ := aes.NewCipher(key)
	gcm, _ := cipher.NewGCM(block)
	ns := gcm.NonceSize()
	pt, err := gcm.Open(nil, data[:ns], data[ns:], nil)
	return string(pt), err
}

func loadOrCreateKey(path string) ([]byte, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		key := make([]byte, 32)
		io.ReadFull(rand.Reader, key)
		os.WriteFile(path, key, 0600)
		return key, nil
	}
	return os.ReadFile(path)
}
```

### 17.3 存储实现

```go
package secret

import ("database/sql"; _ "github.com/mattn/go-sqlite3")

type Storage interface {
	Save(key, value string) error
	Load(key string) (string, error)
	Delete(key string) error
}

type FileStorage struct{ db *sql.DB }

func NewFileStorage(path string) (*FileStorage, error) {
	db, _ := sql.Open("sqlite3", path)
	db.Exec(`CREATE TABLE IF NOT EXISTS secrets (key TEXT PRIMARY KEY, value TEXT NOT NULL)`)
	return &FileStorage{db}, nil
}

func (s *FileStorage) Save(k, v string) error {
	_, err := s.db.Exec("INSERT OR REPLACE INTO secrets (key, value) VALUES (?,?)", k, v)
	return err
}

func (s *FileStorage) Load(k string) (string, error) {
	var v string
	err := s.db.QueryRow("SELECT value FROM secrets WHERE key=?", k).Scan(&v)
	return v, err
}

func (s *FileStorage) Delete(k string) error {
	_, err := s.db.Exec("DELETE FROM secrets WHERE key=?", k)
	return err
}
```

## 18. 超时控制模块设计

```go
package timeout

import ("context"; "fmt"; "time")

type Config struct {
	TaskMaxDuration      time.Duration `yaml:"task_max_duration"`
	StepMaxDuration      time.Duration `yaml:"step_max_duration"`
	LLMAPITimeout        time.Duration `yaml:"llm_api_timeout"`
	MCPToolTimeout       time.Duration `yaml:"mcp_tool_timeout"`
	TestExecutionTimeout time.Duration `yaml:"test_execution_timeout"`
}

func DefaultConfig() Config {
	return Config{2 * time.Hour, 30 * time.Minute, 60 * time.Second, 120 * time.Second, 10 * time.Minute}
}

type Controller struct{ config Config }

func NewController(cfg Config) *Controller { return &Controller{cfg} }

func (c *Controller) WithTaskTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, c.config.TaskMaxDuration)
}

func (c *Controller) WithLLMTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, c.config.LLMAPITimeout)
}

func (c *Controller) ExecuteWithTimeout(ctx context.Context, fn func(ctx context.Context) error, timeout time.Duration) error {
	tCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	errCh := make(chan error, 1)
	go func() { errCh <- fn(tCtx) }()
	select {
	case err := <-errCh: return err
	case <-tCtx.Done(): return fmt.Errorf("timeout after %v", timeout)
	}
}
```

## 19. 项目分析模块设计

### 19.1 分析器实现

```go
package analyzer

import ("os"; "path/filepath"; "strings"; "encoding/json")

type ProjectType string
const (TypeWeb ProjectType = "web"; TypeAPI = "api"; TypeCLI = "cli"; TypeLib = "library")

type ProjectStructure struct {
	Type         ProjectType         `json:"type"`
	Language     string              `json:"language"`
	Framework    string              `json:"framework"`
	EntryPoints  []string            `json:"entry_points"`
	ConfigFiles  []string            `json:"config_files"`
	TestFiles    []string            `json:"test_files"`
	CodeStyle    *CodeStyle          `json:"code_style"`
	Dependencies map[string][]string `json:"dependencies"`
}

type CodeStyle struct {
	IndentSize int    `json:"indent_size"`
	Linter     string `json:"linter"`
	Formatter  string `json:"formatter"`
}

type ProjectAnalyzer struct{ rootPath string }
func New(rootPath string) *ProjectAnalyzer { return &ProjectAnalyzer{rootPath} }

func (a *ProjectAnalyzer) Analyze() (*ProjectStructure, error) {
	s := &ProjectStructure{Dependencies: make(map[string][]string)}
	s.Language = a.detectLanguage()
	s.Framework = a.detectFramework()
	s.Type = a.detectProjectType()
	s.CodeStyle = a.analyzeCodeStyle()
	a.collectFiles(s)
	return s, nil
}

func (a *ProjectAnalyzer) detectLanguage() string {
	checks := map[string]string{
		"go.mod": "go", "package.json": "javascript", "requirements.txt": "python",
		"Cargo.toml": "rust", "pom.xml": "java", "composer.json": "php",
	}
	for f, lang := range checks {
		if _, err := os.Stat(filepath.Join(a.rootPath, f)); err == nil { return lang }
	}
	return "unknown"
}

func (a *ProjectAnalyzer) detectFramework() string {
	lang := a.detectLanguage()
	if lang == "javascript" {
		data, _ := os.ReadFile(filepath.Join(a.rootPath, "package.json"))
		var pkg struct{ Dependencies map[string]string `json:"dependencies"` }
		json.Unmarshal(data, &pkg)
		for f := range pkg.Dependencies {
			if f == "react" { return "react" }
			if f == "vue" { return "vue" }
			if f == "next" { return "nextjs" }
			if f == "express" { return "express" }
		}
	}
	if lang == "go" {
		data, _ := os.ReadFile(filepath.Join(a.rootPath, "go.mod"))
		c := string(data)
		if strings.Contains(c, "gin-gonic") { return "gin" }
		if strings.Contains(c, "labstack/echo") { return "echo" }
	}
	return "unknown"
}

func (a *ProjectAnalyzer) detectProjectType() ProjectType {
	checks := map[string]ProjectType{"cmd/": TypeCLI, "pages/": TypeWeb, "routes/": TypeAPI, "api/": TypeAPI}
	for p, t := range checks {
		if _, err := os.Stat(filepath.Join(a.rootPath, p)); err == nil { return t }
	}
	return TypeLib
}

func (a *ProjectAnalyzer) analyzeCodeStyle() *CodeStyle {
	style := &CodeStyle{}
	for _, p := range []string{".eslintrc", ".eslintrc.js", ".eslintrc.json"} {
		if _, err := os.Stat(filepath.Join(a.rootPath, p)); err == nil { style.Linter = "eslint"; break }
	}
	if _, err := os.Stat(filepath.Join(a.rootPath, ".prettierrc")); err == nil { style.Formatter = "prettier" }
	if _, err := os.Stat(filepath.Join(a.rootPath, ".golangci.yml")); err == nil { style.Linter = "golangci-lint" }
	return style
}

func (a *ProjectAnalyzer) collectFiles(s *ProjectStructure) {
	filepath.Walk(a.rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() { return nil }
		rel, _ := filepath.Rel(a.rootPath, path)
		if strings.HasSuffix(rel, "_test.go") || strings.Contains(rel, ".test.") { s.TestFiles = append(s.TestFiles, rel) }
		if isConfigFile(rel) { s.ConfigFiles = append(s.ConfigFiles, rel) }
		return nil
	})
}

func isConfigFile(p string) bool {
	for _, c := range []string{".eslintrc",".prettierrc","tsconfig.json","vite.config.js","tailwind.config.js"} {
		if strings.HasSuffix(p, c) { return true }
	}
	return false
}
```

## 20. 错误诊断模块设计

### 20.1 诊断引擎

```go
package diagnosis

import ("strings"; "regexp")

type ErrorCategory string
const (ECLLM ErrorCategory="llm"; ECMCP="mcp"; ECTest="test"; ECLint="lint";
      ECTimeout="timeout"; ECToken="token"; ECUnknown="unknown")

type Diagnosis struct {
	Error error; Category ErrorCategory; Cause string
	Suggestions []string; RelatedDocs []string
}

type Engine struct{ kb *KnowledgeBase }
func NewEngine() *Engine { return &Engine{NewKnowledgeBase()} }

func (e *Engine) Diagnose(err error) *Diagnosis {
	cat := classifyError(err)
	return &Diagnosis{
		Error: err, Category: cat, Cause: analyzeCause(cat),
		Suggestions: e.kb.Lookup(cat, err.Error()),
		RelatedDocs: findRelatedDocs(cat),
	}
}

func classifyError(err error) ErrorCategory {
	m := err.Error()
	switch {
	case strings.Contains(m,"rate limit"), strings.Contains(m,"api key"): return ECLLM
	case strings.Contains(m,"token") && strings.Contains(m,"exceed"): return ECToken
	case strings.Contains(m,"context length"): return ECToken
	case strings.Contains(m,"mcp"), strings.Contains(m,"tool"): return ECMCP
	case strings.Contains(m,"test"), strings.Contains(m,"assertion"): return ECTest
	case strings.Contains(m,"lint"), strings.Contains(m,"eslint"): return ECLint
	case strings.Contains(m,"timeout"): return ECTimeout
	default: return ECUnknown
	}
}

func analyzeCause(cat ErrorCategory) string {
	c := map[ErrorCategory]string{ECLLM:"LLM API 调用失败", ECMCP:"MCP 工具调用失败",
		ECTest:"测试用例执行失败", ECLint:"静态分析发现代码问题",
		ECTimeout:"操作超时", ECToken:"Token 数量超过限制"}
	if s, ok := c[cat]; ok { return s }
	return "未知错误"
}

func findRelatedDocs(cat ErrorCategory) []string {
	docs := map[ErrorCategory][]string{
		ECLLM: {"https://platform.openai.com/docs/guides/error-codes"},
		ECTimeout: {"https://pkg.go.dev/context#WithTimeout"},
		ECToken: {"https://platform.openai.com/tokenizer"},
	}
	return docs[cat]
}
```

### 20.2 知识库

```go
package diagnosis

import "regexp"

type KnowledgeEntry struct {
	Pattern *regexp.Regexp; Suggestions []string
}

type KnowledgeBase struct{ entries []KnowledgeEntry }

func NewKnowledgeBase() *KnowledgeBase {
	kb := &KnowledgeBase{}
	kb.entries = []KnowledgeEntry{
		{regexp.MustCompile(`(?i)rate limit`), []string{"指数退避重试","降低调用频率","升级 API Key"}},
		{regexp.MustCompile(`(?i)token.*exceed`), []string{"压缩上下文","减少文件数量","使用增量传递"}},
		{regexp.MustCompile(`(?i)permission denied`), []string{"检查文件权限","检查 API Key 权限"}},
		{regexp.MustCompile(`(?i)connection refused`), []string{"检查服务状态","检查端口配置"}},
	}
	return kb
}

func (kb *KnowledgeBase) Lookup(cat ErrorCategory, errMsg string) []string {
	for _, e := range kb.entries {
		if e.Pattern.MatchString(errMsg) { return e.Suggestions }
	}
	return []string{"查看详细错误日志","检查配置","确认依赖服务正常"}
}
```

## 21. 沙箱执行模块设计

```go
package sandbox

import ("context"; "fmt"; "os/exec"; "path/filepath"; "strings"; "time")

type Config struct {
	AllowedPaths      []string `yaml:"allowed_paths"`
	AllowedCommands   []string `yaml:"allowed_commands"`
	MaxMemoryMB       int64    `yaml:"max_memory_mb"`
	MaxCpuTimeSeconds int      `yaml:"max_cpu_time_seconds"`
	NetworkAccess     bool     `yaml:"network_access"`
	WorkDir           string   `yaml:"work_dir"`
}

func DefaultConfig() Config {
	return Config{
		AllowedPaths: []string{".","./src","./test"},
		AllowedCommands: []string{"go","npm","pnpm","yarn","node","python","python3","make","git","ls","cat","grep"},
		MaxMemoryMB: 1024, MaxCpuTimeSeconds: 300, NetworkAccess: false, WorkDir: ".",
	}
}

type Executor struct{ config Config }
type Result struct{ Stdout string; ExitCode int; Duration time.Duration }

func New(cfg Config) *Executor { return &Executor{cfg} }

func (e *Executor) Execute(ctx context.Context, cmd string, args []string) (*Result, error) {
	if !e.isAllowed(cmd) { return nil, fmt.Errorf("command %q not allowed", cmd) }
	for _, a := range args {
		if filepath.IsAbs(a) && !e.isPathAllowed(a) { return nil, fmt.Errorf("path %q not allowed", a) }
	}
	
	tCtx, cancel := context.WithTimeout(ctx, time.Duration(e.config.MaxCpuTimeSeconds)*time.Second)
	defer cancel()
	
	c := exec.CommandContext(tCtx, cmd, args...)
	c.Dir = e.config.WorkDir
	start := time.Now()
	output, err := c.CombinedOutput()
	code := 0
	if err != nil { if ee, ok := err.(*exec.ExitError); ok { code = ee.ExitCode() } else { return nil, err } }
	return &Result{string(output), code, time.Since(start)}, nil
}

func (e *Executor) isAllowed(cmd string) bool {
	c := filepath.Base(cmd)
	for _, a := range e.config.AllowedCommands { if c == a { return true } }
	return false
}

func (e *Executor) isPathAllowed(path string) bool {
	abs, _ := filepath.Abs(path)
	for _, a := range e.config.AllowedPaths {
		aa, _ := filepath.Abs(a)
		if strings.HasPrefix(abs, aa) { return true }
	}
	return false
}
```

## 22. 更新后的项目结构

```
autodev-agent/
├── cmd/
│   ├── root.go
│   ├── run.go
│   ├── config.go
│   ├── status.go
│   └── interactive.go
├── internal/
│   ├── agent/          # 核心 Agent 模块
│   ├── orchestrator/   # Agent 编排器
│   ├── memory/         # 记忆系统
│   ├── session/        # 会话存储
│   ├── mcp/            # MCP 工具集成
│   ├── statemachine/   # 状态机
│   ├── llm/            # LLM 提供商
│   ├── context/        # 上下文管理
│   ├── git/            # Git 操作
│   ├── validator/      # 验收检查
│   ├── task/           # 任务定义
│   ├── config/         # 配置管理
│   ├── confirmation/   # [新增] 人工确认
│   ├── progress/       # [新增] 进度反馈
│   ├── audit/          # [新增] 审计日志
│   ├── secret/         # [新增] 密钥管理
│   ├── timeout/        # [新增] 超时控制
│   ├── diagnosis/      # [新增] 错误诊断
│   ├── sandbox/        # [新增] 沙箱执行
│   ├── analyzer/       # [新增] 项目分析
│   └── registry/       # [新增] 自定义 Agent 注册表
├── pkg/
│   ├── utils/          # 工具函数
│   └── prompt/         # Prompt 模板
├── configs/
│   └── autodev.yaml
├── go.mod
├── go.sum
├── Makefile
└── README.md
```

## 23. 更新后的架构图

```
┌──────────────────────────────────────────────────────────────────┐
│                          CLI Interface                             │
│   (cobra + Progress Reporter + Confirmation + Diagnosis Engine)    │
└────────────────────────────┬─────────────────────────────────────┘
                             │
┌────────────────────────────▼─────────────────────────────────────┐
│                       Task Orchestrator                            │
│   (State Machine + Timeout Controller + Audit Logger + Analyzer)   │
└──┬─────┬──────┬───────┬────────┬────────┬──────┬────────┬───────┘
   │     │      │       │        │        │      │        │
┌──▼──┐┌──▼──┐┌──▼──┐┌──▼───┐┌───▼────┐┌──▼──┐┌─▼─────┐┌─▼──────┐
│Parser││Coder ││Tester││Checker││MemorySys││Recovery││Session ││  MCP   │
│Agent ││Agent ││Agent ││Agent  ││         ││Agent  ││Store   ││ Client  │
│+Analyzer│+Sandbox│      │+Diagnosis│         │+Diagnosis││+Snapshots│
└──────┴┴──────┴┴──────┴┴───────┴┴─────────┴┴───────┴┴────────┴┴────────┘

Cross-Cutting Modules (全局调用):
┌────────────────┐  ┌────────────────┐  ┌────────────────┐
│ Confirmation   │  │ Progress       │  │ Audit Logger   │
└────────────────┘  └────────────────┘  └────────────────┘
┌────────────────┐  ┌────────────────┐  ┌────────────────┐
│ Secret Manager │  │ Timeout Ctrl   │  │ Diagnosis      │
│ (AES-GCM)      │  │ (context)      │  │ Engine         │
└────────────────┘  └────────────────┘  └────────────────┘
┌────────────────┐  ┌────────────────┐
│ Sandbox        │  │ ProjectAnalyzer│
│ Executor       │  │ (stack/style)  │
└────────────────┘  └────────────────┘

Data Storage (SQLite):
┌───────────────┐ ┌───────────────┐ ┌───────────────┐ ┌──────────────┐
│ Memory DB     │ │ Session DB    │ │ Audit DB      │ │ Secrets DB   │
│ (FTS5 Search) │ │ (Snapshots)   │ │ (Query/Export)│ │ (AES-GCM)    │
└───────────────┘ └───────────────┘ └───────────────┘ └──────────────┘
```

## 24. 配置更新

```yaml
# autodev.yaml (新增模块配置)
version: "1.0"

# ... 原有配置 ...

confirmation:
  enabled: true
  interactive: false
  auto_approve: false

progress:
  enabled: true
  format: "terminal"
  verbose: false

audit:
  enabled: true
  db_path: "~/.autodev/audit.db"
  retention_days: 30

secret:
  enabled: true
  db_path: "~/.autodev/secrets.db"
  algorithm: "aes-gcm"

timeout:
  task_max_duration: "2h"
  step_max_duration: "30m"
  llm_api_timeout: "60s"
  mcp_tool_timeout: "120s"
  test_execution_timeout: "10m"

sandbox:
  enabled: true
  max_memory_mb: 1024
  max_cpu_time_seconds: 300
  network_access: false
  allowed_commands:
    - go - npm - python - git - make

diagnosis:
  enabled: true

analyzer:
  enabled: true
  auto_detect: true

custom_agents:
  enabled: true
  directories:
    - ~/.autodev/agents       # 全局自定义 Agent 目录
    - ./.autodev/agents        # 项目级自定义 Agent 目录
  hot_reload: true            # 支持热重载
  prompt_templates:
    max_size_kb: 128          # Prompt 模板最大尺寸
```

## 25. 自定义 Agent 设计

### 25.1 Agent 定义文件

```yaml
# .autodev/agents/review-agent.yaml
name: review-agent
type: custom
description: "代码审查 Agent，自动审查代码质量"

behavior:
  prompt_template: "templates/review.md"
  model: "gpt-4"
  temperature: 0.5
  max_tokens: 4096
  system_prompt: "你是一个专业的代码审查员，擅长发现潜在 Bug 和安全漏洞"

capabilities:
  mcp_tools:
    - read_file
    - grep_search
    - bash_execute
  allowed_paths:
    - "."
    - "./src"

routing:
  triggers:
    - type: keyword
      pattern: "review|code quality|静态分析"
      priority: 10
  auto_handle: true

validation:
  quality_gates:
    - type: "no_security_issue"
      severity: "high"
  max_execution_time: "5m"
```

### 25.2 Prompt 模板

```markdown
# .autodev/agents/templates/review.md

你是一个专业的代码审查 Agent。请基于以下信息完成代码审查任务。

## 项目信息
- 语言：{{.Project.Language}}
- 框架：{{.Project.Framework}}
- Linter：{{.Project.CodeStyle.Linter}}

## 任务描述
{{.Task.Description}}

## 相关文件
{{range .Task.Files}}
- {{.}}
{{end}}

## 审查要求
{{.Agent.Behavior.SystemPrompt}}

请逐行审查代码，重点关注：
1. 潜在 Bug
2. 安全漏洞
3. 性能问题
4. 代码规范
5. 可维护性

输出格式：
| 文件 | 行号 | 问题类型 | 严重等级 | 描述 |
```

### 25.3 Agent 注册表

```go
package registry

import (
    "fmt"
    "os"
    "path/filepath"
    "sync"
    "gopkg.in/yaml.v3"
)

type AgentDefinition struct {
    Name         string            `yaml:"name"`
    Type         string            `yaml:"type"`
    Description  string            `yaml:"description"`
    Behavior     AgentBehavior     `yaml:"behavior"`
    Capabilities AgentCapabilities `yaml:"capabilities"`
    Routing      AgentRouting      `yaml:"routing"`
    Validation   AgentValidation   `yaml:"validation"`
}

type AgentBehavior struct {
    PromptTemplate string  `yaml:"prompt_template"`
    Model          string  `yaml:"model"`
    Temperature    float64 `yaml:"temperature"`
    MaxTokenss     int     `yaml:"max_tokens"`
    SystemPrompt   string  `yaml:"system_prompt"`
}

type AgentCapabilities struct {
    MCPTools     []string `yaml:"mcp_tools"`
    AllowedPaths []string `yaml:"allowed_paths"`
}

type AgentRouting struct {
    Triggers   []RoutingTrigger `yaml:"triggers"`
    AutoHandle bool             `yaml:"auto_handle"`
}

type RoutingTrigger struct {
    Type     string `yaml:"type"`
    Pattern  string `yaml:"pattern"`
    Priority int    `yaml:"priority"`
}

type AgentValidation struct {
    QualityGates     []QualityGate `yaml:"quality_gates"`
    MaxExecutionTime string        `yaml:"max_execution_time"`
}

type QualityGate struct {
    Type     string `yaml:"type"`
    Severity string `yaml:"severity"`
}

type AgentRegistry struct {
    mu     sync.RWMutex
    agents map[string]*AgentDefinition
    dirs   []string
}

func NewAgentRegistry(dirs []string) *AgentRegistry {
    return &AgentRegistry{
        agents: make(map[string]*AgentDefinition),
        dirs:   dirs,
    }
}

func (r *AgentRegistry) LoadAgents() error {
    r.mu.Lock()
    defer r.mu.Unlock()

    for _, dir := range r.dirs {
        if _, err := os.Stat(dir); os.IsNotExist(err) {
            continue
        }

        files, err := filepath.Glob(filepath.Join(dir, "*.yaml"))
        if err != nil {
            return err
        }

        for _, f := range files {
            data, err := os.ReadFile(f)
            if err != nil {
                continue
            }

            def := &AgentDefinition{}
            if err := yaml.Unmarshal(data, def); err != nil {
                continue
            }

            r.agents[def.Name] = def
        }
    }
    return nil
}

func (r *AgentRegistry) Get(name string) (*AgentDefinition, bool) {
    r.mu.RLock()
    defer r.mu.RUnlock()
    def, ok := r.agents[name]
    return def, ok
}

func (r *AgentRegistry) List() []*AgentDefinition {
    r.mu.RLock()
    defer r.mu.RUnlock()
    defs := make([]*AgentDefinition, 0, len(r.agents))
    for _, def := range r.agents {
        defs = append(defs, def)
    }
    return defs
}

func (r *AgentRegistry) Register(def *AgentDefinition) error {
    r.mu.Lock()
    defer r.mu.Unlock()
    if _, exists := r.agents[def.Name]; exists {
        return fmt.Errorf("agent %s already registered", def.Name)
    }
    r.agents[def.Name] = def
    return nil
}
```

### 25.4 自定义 Agent 执行器

```go
package agent

import (
    "bytes"
    "context"
    "html/template"
    "os"
    "path/filepath"
    "time"
)

type CustomAgent struct {
    BaseAgent
    definition *registry.AgentDefinition
    baseDir    string
}

func NewCustomAgent(
    def *registry.AgentDefinition,
    baseDir string,
    memory *MemorySystem,
    session *SessionStore,
    mcpClient *MCPClient,
    llm LLMProvider,
) *CustomAgent {
    return &CustomAgent{
        BaseAgent: BaseAgent{
            type_:       AgentType("custom/" + def.Name),
            name:        def.Name,
            memory:      memory,
            session:     session,
            mcpClient:   mcpClient,
            llmProvider: llm,
        },
        definition: def,
        baseDir:    baseDir,
    }
}

func (a *CustomAgent) Execute(ctx context.Context, task *Task) (*Result, error) {
    promptText, err := a.loadPromptTemplate(task)
    if err != nil {
        return nil, err
    }

    llmCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
    defer cancel()

    response, err := a.llmProvider.Complete(llmCtx, promptText, &LLMOptions{
        Model:       a.definition.Behavior.Model,
        Temperature: a.definition.Behavior.Temperature,
        MaxTokens:   a.definition.Behavior.MaxTokenss,
    })
    if err != nil {
        return nil, err
    }

    return &Result{Success: true, Output: response}, nil
}

func (a *CustomAgent) loadPromptTemplate(task *Task) (string, error) {
    tmplPath := filepath.Join(a.baseDir, a.definition.Behavior.PromptTemplate)
    tmpl, err := os.ReadFile(tmplPath)
    if err != nil {
        return "", err
    }

    t, err := template.New("prompt").Parse(string(tmpl))
    if err != nil {
        return "", err
    }

    data := map[string]any{
        "Task":    task,
        "Agent":   a.definition,
        "Project": a.projectStructure,
    }

    var buf bytes.Buffer
    if err := t.Execute(&buf, data); err != nil {
        return "", err
    }
    return buf.String(), nil
}

func (a *CustomAgent) CanHandle(task *Task) bool {
    for _, trigger := range a.definition.Routing.Triggers {
        switch trigger.Type {
        case "keyword":
            // 匹配关键词
            // 实现...
        }
    }
    return false
}

func (a *CustomAgent) MCPTools() []string {
    return a.definition.Capabilities.MCPTools
}
```

### 25.5 Agent CLI 命令

```go
package cmd

import (
    "fmt"
    "os"
    "path/filepath"

    "github.com/spf13/cobra"
)

var agentCmd = &cobra.Command{
    Use:   "agent",
    Short: "自定义 Agent 管理",
}

var agentListCmd = &cobra.Command{
    Use:   "list",
    Short: "列出所有可用 Agent",
    Run: func(cmd *cobra.Command, args []string) {
        reg := loadAgentRegistry()
        builtins := []string{"parser", "coder", "tester", "checker", "recovery"}

        fmt.Println("内置 Agent:")
        for _, n := range builtins {
            fmt.Printf("  - %s\n", n)
        }

        fmt.Println("\n自定义 Agent:")
        for _, def := range reg.List() {
            fmt.Printf("  - %s (%s)\n", def.Name, def.Description)
        }
    },
}

var agentNewCmd = &cobra.Command{
    Use:   "new <name>",
    Short: "创建新的自定义 Agent 模板",
    Args:  cobra.ExactArgs(1),
    Run: func(cmd *cobra.Command, args []string) {
        name := args[0]
        agentDir := filepath.Join(".autodev", "agents")
        os.MkdirAll(filepath.Join(agentDir, "templates"), 0755)

        // 生成 YAML 定义文件
        yamlContent := fmt.Sprintf(`name: %s
type: custom
description: "自定义 %s Agent"

behavior:
  prompt_template: "templates/prompt.md"
  model: "gpt-4"
  temperature: 0.5
  max_tokens: 4096
  system_prompt: "你是..."

capabilities:
  mcp_tools:
    - read_file
  allowed_paths:
    - "."

routing:
  triggers:
    - type: keyword
      pattern: "%s"
      priority: 10
  auto_handle: true

validation:
  quality_gates: []
  max_execution_time: "5m"
`, name, name, name)

        os.WriteFile(filepath.Join(agentDir, name+".yaml"), []byte(yamlContent), 0644)

        // 生成 Prompt 模板
        promptContent := fmt.Sprintf(`你是 {{.Agent.Behavior.SystemPrompt}}

## 任务
{{.Task.Description}}

## 项目信息
- 语言：{{.Project.Language}}
- 框架：{{.Project.Framework}}

请执行任务并输出结果。
`)
        os.WriteFile(filepath.Join(agentDir, "templates", "prompt.md"), []byte(promptContent), 0644)

        fmt.Printf("✓ Created custom agent: %s\n", name)
        fmt.Printf("  定义文件: %s\n", filepath.Join(agentDir, name+".yaml"))
        fmt.Printf("  Prompt 模板: %s\n", filepath.Join(agentDir, "templates", "prompt.md"))
    },
}

func init() {
    agentCmd.AddCommand(agentListCmd)
    agentCmd.AddCommand(agentNewCmd)
    rootCmd.AddCommand(agentCmd)
}
```

### 25.6 使用示例

```bash
# 1. 查看所有可用 Agent
autodev agent list

# 输出:
# 内置 Agent:
#   - parser
#   - coder
#   - tester
#   - checker
#   - recovery
#
# 自定义 Agent:
#   - review-agent (代码审查 Agent)
#   - deploy-agent (部署 Agent)

# 2. 创建新的自定义 Agent
autodev agent new security-scanner

# 3. 编辑 Agent 定义
vim .autodev/agents/security-scanner.yaml

# 4. 使用自定义 Agent
autodev run "请扫描代码中的安全漏洞"
# Orchestrator 自动匹配到 security-scanner (keyword 匹配 "安全")

# 5. 热重载自定义 Agent（编辑后无需重启）
autodev config reload agents
```

## 26. 数据流图

### 26.1 主数据流

```
用户输入任务描述
       │
       ▼
   ┌────────┐
   │   CLI  │ 解析命令行参数
   └───┬────┘
       │ taskDescription
       ▼
┌──────────────┐
│ Orchestrator │ 初始化组件
└──────┬───────┘
       │
       ├─► 加载记忆系统 (SQLite FTS5)
       ├─► 恢复会话状态 (断点续传)
       ├─► 分析项目结构 (语言/框架/风格)
       ├─► 加载自定义 Agent (YAML 注册表)
       │
       ▼
════════════ 状态机驱动执行 ════════════

 ┌─────────┐
 │  IDLE   │
 └────┬────┘ start
      ▼
 ┌───────────┐    ┌────────────┐
 │ PARSING   │───►│  ANALYZED  │
 │ParserAgent│    │TaskPlan生成│
 └─────┬─────┘    └─────┬──────┘
       │  sub-tasks     │ plan_ready
       │        ┌───────▼───────┐
       │        │  DEVELOPING   │
       │        │  CoderAgent   │── 多轮 LLM 对话生成/修改代码
       │        │  + MCP Tools  │── 写入文件 + Git 快照
       │        └───────┬───────┘
       │                │ code_done
       │        ┌───────▼───────┐
       │        │   TESTING     │
       │        │  TesterAgent  │── 自动生成测试用例
       │        │               │── 执行测试 + 覆盖率收集
       │        └───────┬───────┘
       │                │ test_result
       │        ┌───────▼───────┐
       │        │   CHECKING    │
       │        │  CheckerAgent │── 覆盖率门禁 (80%+)
       │        │               │── 静态分析 (ESLint/Pylint)
       │        │               │── 质量门禁验证
       │        └───────┬───────┘
       │                │ check_result
       │    ┌───────────┼───────────┐
       │    │ passed    │ failed    │
       │    ▼           ▼           │
       │ ┌────────┐ ┌──────────┐   │
       │ │COMPLETED││RECOVERING│   │
       │ └───┬────┘ │ Retry≤3  │   │
       │     │      └────┬─────┘   │
       │     │           │ fix    │
       │     │      ┌────▼────┐   │
       │     │      │ROLLBACK │   │
       │     │      │恢复快照  │   │
       │     │      └────┬────┘   │
       │     │           │ retry  │
       │     │      ┌────┴────┐   │
       │     │      │REDEVELOP│───┘
       │     │      └─────────┘
       │     │
       │     │ create_pr
       │     ▼
       │ ┌─────────┐
       └─┤PR_DONE   │ (人工确认)
         │ PR创建   │── 生成标题/描述/变更摘要
         └─────────┘

       │
       ▼
════════════ 执行完成 ════════════════
       │
       ├─► 保存会话 (SQLite)
       ├─► 更新记忆系统
       ├─► 写入审计日志
       │
       ▼
      输出 PR URL + 执行报告
```

### 26.2 Agent 间数据传递

```
Parser Agent
  │
  ├─► TaskPlan{sub-tasks, dependencies}
  │
  ▼
Coder Agent (读取 TaskPlan，按顺序执行子任务)
  │
  ├─► CodeChanges[]{file, oldCode, newCode, diff}
  │
  ▼
Tester Agent (读取 CodeChanges)
  │
  ├─► TestResult{passed, failed, coverage}
  │
  ▼
Checker Agent (读取 TestResult + CodeChanges)
  │
  ├─► CheckResult{pass, violations, gates}
  │
  ▼
Git / PR Module (读取 CodeChanges + CheckResult)
  │
  ├─► PullRequest{title, description, files, url}
```

## 27. Token 管理器详细设计

### 27.1 Token 预算控制

```go
package llm

type TokenManager struct {
    budget    int       // 模型上下文窗口上限
    used      int       // 已使用 Token 数
    remaining int       // 剩余可用
    model     string    // 当前模型
}

func NewTokenManager(model string) *TokenManager {
    budget := modelContextLimit(model)
    return &TokenManager{
        budget:    budget,
        remaining: budget,
        model:     model,
    }
}

func (m *TokenManager) CanUse(count int) bool {
    return m.used+count <= m.budget
}

func (m *TokenManager) Use(count int) error {
    if !m.CanUse(count) {
        return fmt.Errorf("token budget exceeded: need %d, have %d",
            count, m.remaining)
    }
    m.used += count
    m.remaining = m.budget - m.used
    return nil
}
```

### 27.2 上下文智能截断

```go
package llm

// ContextBuilder 构建 LLM 对话内容，自动管理 Token 预算
type ContextBuilder struct {
    mgr      *TokenManager
    messages []Message
    files    []FileInfo
    history  []TaskResult
}

func NewContextBuilder(mgr *TokenManager) *ContextBuilder {
    return &ContextBuilder{mgr: mgr}
}

func (b *ContextBuilder) WithSystemPrompt(prompt string) *ContextBuilder {
    b.messages = append([]Message{{Role: "system", Content: prompt}}, b.messages...)
    return b
}

func (b *ContextBuilder) WithFileContent(file FileInfo) *ContextBuilder {
    b.files = append(b.files, file)
    return b
}

func (b *ContextBuilder) Build() []Message {
    // 按优先级排序文件:
    //   1. 已修改文件 (最高)
    //   2. 相关文件 (中等)
    //   3. 全部文件 (最低)
    sorted := b.sortByPriority(b.files)

    result := make([]Message, 0, len(b.messages)+len(sorted))
    result = append(result, b.messages...)

    // 逐个添加文件内容，超出 Token 预算时截断
    for _, f := range sorted {
        content := f.Content
        if b.mgr.TokensUsed()+countTokens(content) > b.mgr.Budget()*0.8 {
            // 剩余预算不足，使用文件摘要
            content = summarizeFile(f)
        }
        if !contentTruncated && countTokens(content) > b.remaining() {
            content = truncateToBudget(content, b.remaining())
            contentTruncated = true
        }
        result = append(result, Message{
            Role: "user",
            Content: fmt.Sprintf("## File: %s\n```\n%s\n```", f.Path, content),
        })
    }
    return result
}

// summarizeFile 生成文件摘要（节省 Token）
func summarizeFile(f FileInfo) string {
    return fmt.Sprintf("## %s (line %d-%d, %d bytes)",
        f.Path, f.StartLine, f.EndLine, len(f.Content))
}
```

### 27.3 增量传递策略

```go
package llm

// DiffContext 仅传递变更部分（增量更新）
type DiffContext struct {
    diffs     []FileDiff
    baseFiles []FileInfo  // 仅包含文件元信息，不传内容
}

func (d *DiffContext) ToMessages() []Message {
    msgs := make([]Message, 0)

    // 第一轮：传递项目结构（不包含文件内容）
    structure := "## Project Structure\n"
    for _, f := range d.baseFiles {
        structure += fmt.Sprintf("- %s (%d lines, %s)\n",
            f.Path, f.LineCount, f.Language)
    }
    msgs = append(msgs, Message{Role: "system", Content: structure})

    // 第二轮：仅传递变更部分
    for _, diff := range d.diffs {
        msgs = append(msgs, Message{Role: "user",
            Content: "## Changes in " + diff.Path + "\n```diff\n" + diff.Diff + "\n```"})
    }
    return msgs
}
```

## 28. Parser Agent 详细设计

### 28.1 任务解析流程

```go
package agent

// TaskPlan 分解后的任务计划
type TaskPlan struct {
    ID          string       `json:"id"`
    Type        string       `json:"type"`  // feature|bugfix|refactor|optimize
    SubTasks    []*SubTask   `json:"sub_tasks"`
    Dependencies map[string][]string `json:"dependencies"` // task_id -> [depends_on_ids]
    TechStack   TechStack    `json:"tech_stack"`
    RiskLevel   string       `json:"risk_level"`
}

type SubTask struct {
    ID          string   `json:"id"`
    Title       string   `json:"title"`
    Description string   `json:"description"`
    AgentType   string   `json:"agent_type"`
    Status      string   `json:"status"`
    Files       []string `json:"files"`
}

type TechStack struct {
    Language    string   `json:"language"`
    Framework   string   `json:"framework"`
    Libraries   []string `json:"libraries"`
    DBType      string   `json:"db_type"`
}

func (a *ParserAgent) Execute(ctx context.Context, task *Task) (*Result, error) {
    // 1. 获取项目上下文
    structure := a.projectAnalyzer.Analyze()

    // 2. 构建 Prompt
    prompt := a.buildPrompt(task, structure)

    // 3. 调用 LLM 解析
    resp, err := a.complete(ctx, prompt)
    if err != nil {
        return nil, err
    }

    // 4. 解析 LLM 输出为 TaskPlan
    plan, err := a.parsePlan(resp.Content)
    if err != nil {
        return nil, err
    }

    // 5. 验证计划完整性
    if err := a.validatePlan(plan); err != nil {
        return nil, err
    }

    // 6. 生成依赖图
    plan.Dependencies = a.analyzeDependencies(plan.SubTasks)

    return &Result{
        Success: true,
        Output:  plan,
    }, nil
}
```

### 28.2 解析 Prompt 模板

```markdown
你是一个智能任务分解 Agent。请分析以下任务描述，将其分解为可执行的子任务计划。

## 项目上下文
- 语言: {{.Language}}
- 框架: {{.Framework}}
- 目录结构:
{{range .Files}}- {{.}}
{{end}}

## 任务描述
{{.TaskDescription}}

请输出 JSON 格式的任务计划，包含以下字段:
{
  "type": "feature|bugfix |refactor|optimize",
  "sub_tasks": [
    {
      "id": "step_1",
      "title": "步骤标题",
      "description": "详细描述，包含涉及的文件和代码逻辑"
    }
  ],
  "tech_stack": ["语言/库/框架"],
  "risk_level": "low |medium| high"
}

分解原则:
1. 每个子任务应修改 1-3 个文件
2. 子任务之间尽量解耦
3. 识别文件依赖关系
4. 区分前端、后端、测试等不同领域
```

### 28.3 子任务依赖分析

```go
package agent

func (a *ParserAgent) analyzeDependencies(tasks []*SubTask) map[string][]string {
    deps := make(map[string][]string)

    // 基于文件路径分析依赖
    fileToTasks := make(map[string][]string)
    for _, t := range tasks {
        for _, f := range t.Files {
            fileToTasks[f] = append(fileToTasks[f], t.ID)
        }
    }

    // 共享文件的任务可能存在依赖
    for f, taskIDs := range fileToTasks {
        if len(taskIDs) > 1 {
            // 后定义的任务依赖先定义的任务
            for i := 1; i < len(taskIDs); i++ {
                deps[taskIDs[i]] = append(deps[taskIDs[i]], taskIDs[i-1])
            }
        }
    }

    return deps
}
```

## 29. Coder Agent 详细设计

### 29.1 代码生成策略

```go
package agent

// CodeChange 表示一个代码变更
type CodeChange struct {
    FilePath    string   `json:"file_path"`
    OldCode     string   `json:"old_code"`     // 变更前的代码
    NewCode     string   `json:"new_code"`     // 变更后的代码
    Diff        string   `json:"diff"`         // 差异
    Description string   `json:description"`   // 变更说明
    LineStart   int      `json:"line_start"`
    LineEnd     int      `json:"line_end"`
}

func (a *CoderAgent) Execute(ctx context.Context, task *Task) (*Result, error) {
    // 1. 定位需要修改的文件
    targets, err := a.locateFiles(task)
    if err != nil {
        return nil, err
    }

    // 2. 读取相关文件
    contextBuilder := a.newContextBuilder(task)
    for _, t := range targets {
        code, err := a.mcpClient.CallTool("read_file", map[string]any{"path": t})
        if err != nil { return nil, err }
        contextBuilder.WithFileContent(FileInfo{Path: t, Content: code})
    }

    // 3. 生成代码变更
    changes := make([]*CodeChange, 0)
    for _, target := range targets {
        prompt := a.buildCodePrompt(task, target, contextBuilder)
        resp, err := a.complete(ctx, prompt)
        if err != nil { return nil, err }

        // 解析 LLM 输出为 diff/new code
        change := a.parseCodeResponse(resp.Content, target)
        changes = append(changes, change)
    }

    // 4. 应用代码变更
    for _, change := range changes {
        if err := a.applyChange(change); err != nil {
            return nil, err
        }
    }

    // 5. 创建快照（用于回退）
    a.session.CreateSnapshot(task.ID, &Snapshot{
        Files: a.captureFileStates(changes),
        Description: "After code generation",
    })

    return &Result{Success: true, Output: changes}, nil
}
```

### 29.2 代码生成 Prompt

```markdown
你是一个专业的高级软件工程师。请根据任务描述生成代码变更。

## 现有代码
## File: {{.File}}
{{.Content}}

## 任务要求
{{.TaskDescription}}

## 变更要求
1. 在不破坏现有功能的前提下修改代码
2. 保持代码风格和格式一致性
3. 添加必要的错误处理和边界检查
4. 保持文件缩进和代码风格一致

## 输出格式
请输出完整的修改后代码:
```{{.Language}}
// 完整代码...
```
```

### 29.3 Diff 应用与冲突解决

```go
package agent

func (a *CoderAgent) applyChange(change *CodeChange) error {
    // 方式 1: 直接写入新代码
    err := a.mcpClient.CallTool("write_file", map[string]any{
        "path":    change.FilePath,
        "content": change.NewCode,
    })
    if err != nil {
        return err
    }

    // 方式 2: 应用 PATCH diff（增量更新模式）
    if change.Diff != "" {
        err := applyPatch(change.FilePath, change.Diff)
        if err != nil {
            // diff 应用失败时，回退到完整写入
            return a.mcpClient.CallTool("write_file", map[string]any{
                "path":    change.FilePath,
                "content": change.NewCode,
            })
        }
    }
    return nil
}

// applyPatch 通过 patch 命令应用 Diff
func applyPatch(filePath, diff string) error {
    tmpFile, _ := os.CreateTemp("", "patch-*.diff")
    defer os.Remove(tmpFile.Name())
    tmpFile.Write([]byte(diff))

    cmd := exec.Command("patch", "-p0", "-i", tmpFile.Name())
    cmd.Dir = filepath.Dir(filePath)
    return cmd.Run()
}

// 当多个 Agent 修改同一文件时发生冲突
func resolveConflict(filePath, baseCode, agentACode, agentBCode string) (string, error) {
    // 策略 1: LLM 自动合并
    // 策略 2: 保留两个版本，人工确认
    // TODO: 实现合并逻辑
    return merged, nil
}
```

## 30. Tester Agent 详细设计

### 30.1 测试框架检测

```go
package agent

type TestFramework string

const (
    FrameworkJest        TestFramework = "jest"
    FrameworkVitest      TestFramework = "vitest"
    FrameworkPytest      TestFramework = "pytest"
    FrameworkJUnit       TestFramework = "junit"
    FrameworkGoTest      TestFramework = "go test"
    FrameworkMocha       TestFramework = "mocha"
    FrameworkUnicorn     TestFramework = "unknown"
)

func (a *TesterAgent) detectFramework() TestFramework {
    // 1. 检查 package.json
    if pkg, err := os.ReadFile("package.json"); err == nil {
        if bytes.Contains(pkg, []byte("jest"))        { return FrameworkJest }
        if bytes.Contains(pkg, []byte("vitest"))      { return FrameworkVitest }
        if bytes.Contains(pkg, []byte("mocha"))       { return FrameworkMocha }
    }

    // 2. 检查 requirements.txt / pyproject.toml
    if req, err := os.ReadFile("requirements.txt"); err == nil {
        if bytes.Contains(req, []byte("pytest"))      { return FrameworkPytest }
    }

    // 3. 检查 go.mod
    if _, err := os.Stat("go.mod"); err == nil {
        return FrameworkGoTest
    }

    // 4. 检查 pom.xml
    if _, err := os.Stat("pom.xml"); err == nil {
        return FrameworkJUnit
    }

    return FrameworkUnknown
}
```

### 30.2 测试用例生成

```go
package agent

type TestCase struct {
    Name        string `json:"name"`
    Description string `json:"description"`
    Input       any    `json:"input"`
    Expected    any    `json:"expected"`
    FilePath    string `json:"file_path"`   // 测试文件路径
    Content     string `json:"content"`     // 测试代码
    Type        string `json:"type"`        // "unit", "integration"
}

func (a *TesterAgent) GenerateTests(ctx context.Context, changes []*CodeChange) ([]*TestCase, error) {
    tests := make([]*TestCase, 0)

    for _, change := range changes {
        // 检测被修改的代码涉及哪些函数/类
        functions, err := a.extractFunctions(change.OldCode)
        if err != nil { continue }

        for _, fn := range functions {
            // 为每个函数生成测试用例
            prompt := a.buildTestPrompt(change, fn)
            resp, err := a.complete(ctx, prompt)
            if err != nil { continue }

            testCase := a.parseTestCase(resp.Content, change.FilePath)
            tests = append(tests, &testCase)
        }
    }

    // 写入测试文件
    for _, tc := range tests {
        a.mcpClient.CallTool("write_file", map[string]any{
            "path":    tc.FilePath,
            "content": tc.Content,
        })
    }

    return tests, nil
}

func (a *TesterAgent) buildTestPrompt(change *CodeChange, fn string) string {
    return fmt.Sprintf(`请为以下代码生成单元测试:

## 原始代码
{{.OldCode}}

## 变更说明
{{.Description}}

## 目标函数
%s

## 测试要求
1. 测试正常流程和异常流程
2. 覆盖边界条件
3. 断言清晰准确
4. 使用项目现有的测试框架

## 输出格式
请输出完整的测试代码:
%s
`, fn, getTestFrameworkLanguage(a.currentFramework))
}
```

### 30.3 测试执行与结果分析

```go
package agent

type TestResult struct {
    Passed    int            `json:"passed"`
    Failed    int            `json:"failed"`
    Total     int            `json:"total"`
    Skipped   int            `json:"skipped"`
    Duration  time.Duration `json:"duration"`
    Failures  []*TestFailure `json:"failures"`
    Coverage   *Coverage     `json:"coverage,omitempty"`
}

type TestFailure struct {
    TestName  string `json:"test_name"`
    FilePath  string `json:"file_path"`
    Line      int    `json:"line"`
    Error     string `json:"error"`
    Hint      string `json:"hint"`        // 失败原因解释
}

type Coverage struct {
    Lines      float64 `json:"lines"`
    Branches   float64 `json:"branches"`
    Functions  float64 `json:"functions"`
    Statements float64 `json:"statements"`
}

func (a *TesterAgent) RunTests(ctx context.Context, config TestConfig) (*TestResult, error) {
    // 1. 构建测试命令
    cmd := a.buildTestCommand(config.Framework)

    // 2. 执行测试
    start := time.Now()
    output, err := a.mcpClient.CallTool("bash_execute", map[string]any{
        "command": cmd.Command,
        "args":    cmd.Args,
        "timeout": "10m",
    })

    // 3. 解析测试输出
    result := a.parseTestOutput(output, time.Since(start))

    // 4. 收集覆盖率
    if config.Coverage {
        coverage, err := a.collectCoverage(config.Framework)
        if err == nil {
            result.Coverage = coverage
        }
    }

    // 5. 分析失败原因
    for _, failure := range result.Failures {
        failure.Hin = a.analyzeFailure(failure)
    }

    return result, nil
}

// 测试命令生成
func (a *TesterAgent) buildTestCommand(fw TestFramework) TestCommand {
    switch fw {
    case FrameworkGoTest:
        return TestCommand{Command: "go", Args: []string{"test", "-v", "-cover", "./..."}}
    case FrameworkJest:
        return TestCommand{Command: "npx", Args: []string{"jest", "--coverage"}}
    case FrameworkVitest:
        return TestCommand{Command: "npx", Args: []string{"vitest", "run", "--coverage"}}
    case FrameworkPytest:
        return TestCommand{Command: "pytest", Args: []string{"--cov=.", "--cov-report=json"}}
    default:
        return TestCommand{Command: "npm", Args: []string{"test"}}
    }
}
```

## 31. Checker Agent 详细设计

### 31.1 质量门禁引擎

```go
package validator

// QualityGate 质量门禁
type QualityGate struct {
    ID        string  `json:"id"`
    Name      string  `json:"name"`
    Type      string  `json:"type"`   // "test_pass_rate", "coverage", "lint", "security"
    Threshold float64 `json:"threshold"`
    Severity  string  `json:"severity"` // "block", "warning", "info"
}

type GateResult struct {
    Gate       QualityGate `json:"gate"`
    Passed     bool        `json:"passed"`
    Actual     float64     `json:"actual"`       // 实际值
    Threshold  float64     `json:"threshold"`
    Details    string      `json:"details"`      // 详细信息
}

func (a *CheckerAgent) RunQualityGates(task *Task, testResult *TestResult) (*GateReport, error) {
    report := &GateReport{Passed: true}

    // 1. 测试通过率检查
    passRate := float64(testResult.Passed) / float64(testResult.Total)
    gate := QualityGate{Type: "test_pass_rate", Threshold: 1.0, Severity: "block"}
    if passRate < gate.Threshold {
        report.Add(GateResult{Gate: gate, Failed: true, Actual: passRate,
            Details: fmt.Sprintf("%d/%d tests passed", testResult.Passed, testResult.Total)})
        report.Passed = false
    }

    // 2. 代码覆盖率检查
    if testResult.Coverage != nil {
        coverageGates := []QualityGate{
            {Type: "lines", Threshold: 0.8},
            {Type: "branches", Threshold: 0.7},
            {Type: "functions", Threshold: 0.8},
        }
        for _, g := range coverageGates {
            actual := getCoverageMetric(testResult.Coverage, g.Type)
            if actual < g.Threshold {
                report.Add(GateResult{Gate: g, Failed: true, Actual: actual})
                report.Passed = false
            }
        }
    }

    // 3. 静态分析检查
    lintViolations, err := a.runLint()
    for _, v := range lintViolations {
        if v.Severity == "error" || v.Severity == "critical" {
            report.Add(GateResult{
                Gate:     QualityGate{Type: "lint", Severity: "block"},
                Failed:   true,
                Details:  fmt.Sprintf("%s:%d %s", v.File, v.Line, v.Message),
            })
            report.Passed = false
        }
    }

    return report, nil
}
```

### 31.2 静态分析集成

```go
package validator

// 自动检测项目配置的静态分析工具
func (a *CheckerAgent) detectLintTool() LintTool {
    if _, err := os.Stat(".eslintrc"); err == nil {
        return LintESLint
    }
    if _, err := os.Stat(".golangci.yml"); err == nil {
        return LintGoLangCI
    }
    if _, err := os.Stat("requirements.txt"); err == nil {
        return LintPyLint
    }
    return LintNone
}

func (a *CheckerAgent) runLint() ([]LintViolation, error) {
    tool := a.detectLintTool()

    var cmd string
    switch tool {
    case LintESLint:
        cmd = "npx eslint . --ext .js,.ts,.jsx,.tsx --format json"
    case LintGoLangCI:
        cmd = "golangci-lint run --out-format json"
    case LintPyLint:
        cmd = "pylint --output-format=json ."
    default:
        return nil, nil
    }

    output, err := a.mcpClient.CallTool("bash_execute", map[string]any{"command": cmd})
    if err != nil {
        return nil, err
    }

    return parseLintOutput(output, tool), nil
}
```

## 32. Recovery Agen 详细设计

### 32.1 自动修复策略库

```go
package agent

// FixStrategy 修复策略接口
type FixStrategy interface {
    Name() string
    CanApply(err error) bool           // 是否适用于此错误
    Apply(ctx context.Context, task *Task, err error) (*Result, error)
}

// 策略 1: 基于错误信息的修复
type ErrorMessageFix struct{}
func (s *ErrorMessageFix) Name() string { return "error_message_fix" }
func (s *ErrorMessageFix) CanApply(err error) bool {
    // 检查错误信息是否包含可定位的代码行
    return strings.Contains(err.Error(), "line ") ||
           strings.Contains(err.Error(), "at ")
}
func (s *ErrorMessageFix) Apply(ctx context.Context, task *Task, err error) (*Result, error) {
    // 解析错误信息，定位问题代码行
    location := parseErrorLocation(err.Error())
    // 读取问题代码
    code := readFile(location.File, location.Line)
    // 生成修复
    fixPrompt := fmt.Sprintf("以下代码在第 %d 行导致错误:\n%s\n\n错误信息: %s\n\n请修复代码并输出完整修正后的代码:", location.Line, code, err.Error())
    return generateFix(ctx, fixPrompt)
}

// 策略 2: 基于测试失败的修复
type TestFailureFix struct{}
func (s *TestFailureFix) Name() string { return "test_failure_fix" }
func (s *TestFailureFix) CanApply(err error) bool {
    return strings.Contains(err.Error(), "test failed") ||
           strings.Contains(err.Error(), "FAIL:")
}
func (s *TestFailureFix) Apply(ctx context.Context, task *Task, err error) (*Result, error) {
    // 分析失败的测试用例
    failure := parseTestFailure(err.Error())
    // 生成修复 Prompt
    fixPrompt := fmt.Sprintf("以下代码导致测试 %s 失败:\n\n期望: %s\n实际: %s\n\n请修复代码:",
        failure.TestName, failure.Expected, failure.Actual)
    return generateFix(ctx, fixPrompt)
}

// 策略 3: 基于 Lint 错误的修复
typeLintErrorFix struct{}
func (s *LintErrorFix) Name() string { return "lint_error_fix" }
func (s *LintErrorFix) CanApply(err error) bool {
    return strings.Contains(err.Error(), "eslint") ||
           strings.Contains(err.Error(), "lint")
}

// Recovery Agent 主流程
func (a *RecoveryAgent) Execute(ctx context.Context, task *Task) (*Result, error) {
    err := task.LastError

    if err == nil {
        return nil, fmt.Errorf("no error to recover from")
    }

    // 遍历策略库，尝试修复
    strategies := a.strategies()
    for _, strategy := range strategies {
        if strategy.CanApply(err) {
            a.logger.Info("Applying fix strategy: ", strategy.Name())
            result, err := strategy.Apply(ctx, task, error)
            if err == nil {
                return result, nil
            }
        }
    }

    // 所有策略都失败，触发回退
    return nil, fmt.Errorf("all fix strategies failed")
}
```

### 32.2 重试与回退机制

```go
package orchestrator

func (o *AgentOrchestrator) handleFailure(
    ctx context.Context,
    task *Task,
    err error,
) (*Result, error) {
    task.RetryCount++

    if task.RetryCount <= 3 {
        // 第 1-3 次尝试修复
        recoveryAgent := o.agents[AgentTypeRecovery]
        fixResult, fixErr := recoveryAgent.Execute(ctx, task)
        if fixErr == nil {
            // 修复成功，重新测试
            return o.executeTesting(ctx, task)
        }

        // 修复失败，回退到上一个快照
        o.session.RollbackPrevious(task.ID)
        task.RetryCount++
        return o.executeTesting(ctx, task)
    }

    if task.RetryCount <= 6 {
        // 第 4-6 次尝试替代方案
        alternateResult, alternateErr := o.tryAlternateStrategy(ctx, task)
        if alternateErr == nil {
            return alternateResult, nil
        }

        // 仍然失败，回退并重试
        o.session.RestoreLatestStable(task.ID)
        return o.handleFailure(ctx, task, alternateErr)
    }

    // 超过最大重试次数，报告失败
    return nil, fmt.Errorf("task failed after %d retries, last error: %w",
        task.RetryCount, err)
}

// tryAlternateStrategy 尝试不同的实现策略
func (o *AgentOrchestrator) tryAlternateStrategy(ctx context.Context, task *Task) (*Result, error) {
    // 使用不同的模型或 Prompt 再次生成代码
    alternateAgent := o.router.AlternateRoute(task)
    if alternateAgent == nil {
        return nil, fmt.Errorf("no alternate strategy available")
    }
    return alternateAgent.Execute(ctx, task)
}
```

## 33. Git/PR 模块详细设计

### 33.1 Git 操作

```go
package git

import (
    "fmt"
    "time"
    "os/exec"
    "github.com/go-git/go-git/v5"
)

type Manager struct {
    repo       *git.Repository
    workDir    string
}

func NewManager(workDir string) (*Manager, error) {
    repo, err := git.PlainOpen(workDir)
    return &Manager{repo: repo, workDir: workDir}, err
}

// CreateBranch 创建新分支
func (m *Manager) CreateBranch(taskID, description string) (string, error) {
    branchName := fmt.Sprintf("autodev/%s-%s",
        time.Now().Format("060102"), sanitizeBranch(description))

    cmd := exec.Command("git", "checkout", "-b", branchName)
    cmd.Dir = m.workDir
    if err := cmd.Run(); err != nil {
        return "", err
    }

    return branchName, nil
}

// Commit 提交变更
func (m *Manager) Commit(message string) (string, error) {
    cmd := exec.Command("git", "add", "-A")
    cmd.Dir = m.workDir
    cmd.Run()

    cmd = exec.Command("git", "commit", "-m", message)
    cmd.Dir = m.workDir
    if err := cmd.Run(); err != nil {
        return "", err
    }

    // 获取 commit hash
    cmd = exec.Command("git", "rev-parse", "HEAD")
    cmd.Dir = m.workDir
    output, _ := cmd.Output()
    return string(output), nil
}

// Push 推送到远成
func (m *Manager) Push(branch string) error {
    cmd := exec.Command("git", "push", "-u", "origin", branch)
    cmd.Dir = m.workDir
    return cmd.Run()
}

// GetDiff 获取变更
func (m *Manager) GetDiff(from, to string) (string, error) {
    cmd := exec.Command("git", "diff", "--stat", from, to)
    cmd.Dir = m.workDir
    output, err := cmd.Output()
    return string(output), err
}

// Status 获取当前状态
func (m *Manager) Status() (*git.Status, error) {
    return m.repo.Status()
}
```

### 33.2 PR 创建

```go
package git

// PullRequest 定义
type PullRequest struct {
    Title       string   `json:"title"`
    Description string   `json:"description"`
    Branch      string   `json:"branch"`
    URL         string   `json:"url"`
    Changes     string   `json:"changes"`    // 文件变更统计
    TestResult  string   `json:"test_result"`
}

func (m *Manager) CreatePR(pr PullRequest) (string, error) {
    // 使用 GitHub CLI
    cmd := exec.Command("gh", "pr", "create",
        "--title", pr.Title,
        "--body", pr.Description,
        "--base", "main",
        "--head", pr.Branch,
    )
    cmd.Dir = m.workDir

    output, err := cmd.Output()
    if err != nil {
        return "", err
    }

    return string(output), nil
}

// GeneratePRTitle 生成 PR 标题
func (m *Manager) GeneratePRTitle(task *Task, plan *TaskPlan) string {
    // 根据任务类型和描述生成简洁的标题
    switch task.Type {
    case "feature":
        return fmt.Sprintf("feat: %s", truncate(task.Description, 50))
    case "bugfix":
        return fmt.Sprintf("fix: %s", truncate(task.Description, 50))
    case "refactor":
        return fmt.Sprintf("refactor: %s", truncate(task.Description, 50))
    default:
        return truncate(task.Description, 80)
    }
}

// GeneratePRDescription 生成 PR 描述
func (m *Manager) GeneratePRDescription(task *Task, plan *TaskPlan, testResult *TestResult) string {
    return fmt.Sprintf(`## 🤖 AutoDev Agent Generated PR

### 任务描述
%s

### 完成的工作
- 分解为 %d 个子任务
%s

### 测试结果
- 通过率: %.1f%%
- 覆盖率: %.1f%%
- 执行时间: %s

### 变更文件
%s

### 验证步骤
%s

---
*This PR was generated by AutoDev Agent.*
`, task.Description,
        len(plan.SubTasks),
        formatSubTasks(plan.SubTasks),
        float64(testResult.Passed)/float64(testResult.Total)*100,
        testResult.Coverage,
        testResult.Duration,
        getChangedFiles(),
        getVerificationSteps(),
    )
}
```

## 34. 插件系统

### 34.1 插件接口定义

```go
package plugin

type Plugin interface {
    Name() string
    Version() string
    Description() string

    // 生命周期钩子
    OnTaskStart(ctx context.Context, task *Task) error
    OnTaskComplete(ctx context.Context, result *Result) error
    OnTaskFail(ctx context.Context, err error) error

    OnAgentBefore(ctx context.Context, agentType string, task *Task) error
    OnAgentAfter(ctx context.Context, agentType string, result *Result) error
}

// BasePlugin 提供空实现
type BasePlugin struct{}
func (p *BasePlugin) Name() string                                    { return "" }
func (p *BasePlugin) Version() string                                 { return "v1" }
func (p *BasePlugin) Description() string                             { return "" }
func (p *BasePlugin) OnTaskStart(ctx context.Context, *Task) error    { return nil }
func (p *BasePlugin) OnTaskComplete(ctx context.Context, *Result) error { return nil }
func (p *BasePlugin) OnTaskFail(ctx context.Context, error) error     { return nil }
func (p *BasePlugin) OnAgentBefore(ctx context.Context, string, *Task) error { return nil }
func (p *BasePlugin) OnAgentAfter(ctx context.Context, string, *Result) error { return nil }
```

### 34.2 插件加载器

```go
package plugin

type Manager struct {
    plugins []Plugin
}

func (m *Manager) Register(p Plugin) {
    m.plugins = append(m.plugins, p)
}

func (m *Manager) FireTaskStart(ctx context.Context, task *Task) error {
    for _, p := range m.plugins {
        if err := p.OnTaskStart(ctx, task); err != nil {
            return err
        }
    }
    return nil
}
```

## 35. 错误处理策略

### 35.1 统一错误类型

```go
package errors

type AppError struct {
    Code    string `json:"code"`
    Message string `json:"message"`
    Cause   error  `json:"-"`
    Context map[string]any `json:"context,omitempty"`
}

func (e *AppError) Error() string { return e.Message }

// 预定义的错误代码
const (
    ErrTaskParseFailed  = "task_parse_failed"
    ErrCodeGenFailed    = "code_gen_failed"
    ErrTestFailed       = "test_failed"
    ErrCheckFailed      = "check_failed"
    ErrLLMTimeout       = "llm_timeout"
    ErrTokenExceeded    = "token_exceeded"
    ErrMCPToolFailed    = "mcp_tool_failed"
    // 等等
)

// IsUserError 判断是否是用户可恢复的错误
func IsUserError(err error) bool {
    // 实现...
    return false
}
```

## 36. 测试策略

### 36.1 Mock LLM Provider

```go
// tests/mock_llm.go
package tests

type MockLLMProvider struct {
    responses map[string]string
    calls     []string
}

func (m *MockLLMProvider) Complete(ctx context.Context, messages []llm.Message, opts *llm.Options) (*llm.Response, error) {
    key := messages[0].Content[:50]
    if resp, exists := m.responses[key]; exists {
        m.calls = append(m.calls, key)
        return &llm.Response{Content: resp, Usage: llm.Usage{TotalTokens: 100}}, nil
    }
    return nil, fmt.Errorf("no mock response for: %s", key)
}

func (m *MockLLMProvider) AssertCalled(t *testing.T, key string) {
    for _, c := range m.calls {
        if strings.Contains(c, key) {
            return
        }
    }
    t.Errorf("Expected LLM to be called with %q, but it wasn't", key)
}
```

### 36.2 集成测试示例

```go
func TestAutoParseAndGenerate(t *testing.T) {
    // Setup
    mockLLM := &MockLLMProvider{
        responses: map[string]string{
            "你是智能任务分解 Agent": `{"type":"feature","sub_tasks":[{"id":"step_1","title":"创建登录组件"}]}`,
            "请根据任务描述生成代码": `export default function Login() { return <div>Login</div>; }`,
        },
    }

    orchestrator := NewAgentOrchestrator(mockLLM)
    task := NewTask("实现用户登录功能")

    // Execute
    ctx := context.Background()
    result, err := orchestrator.ExecuteWithAgents(ctx, task)

    // Assert
    assert.NoError(t, err)
    assert.True(t, result.Success)
    mockLLM.AssertCalled(t, "任务分解 Agent")
    mockLLM.AssertCalled(t, "生成代码")
}
```

## 37. 部署方案

### 37.1 Docker Compose

```yaml
# docker-compose.yml
version: "3.8"
services:
  autodev:
    build: .
    volumes:
      - ./:/workspace               # 挂载工作区
      - ~/.autodev:/root/.autodev   # 数据持久化
    environment:
      - OPENAI_API_KEY=${OPENAI_API_KEY}
      - ANTHROPIC_API_KEY=${ANTHROPIC_API_KEY}
      - OLLAMA_URL=http://ollama:11434
    depends_on:
      - ollama

  ollama:
    image: ollama/ollama:latest
    volumes:
      - ollama-data:/root/.ollama
    ports:
      - "11434:11434"

volumes:
  ollama-data:
```

### 37.2 K8s 部署

```yaml
# k8s/autodev-deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: autodev-agent
spec:
  replicas: 1
  template:
    spec:
      containers:
      - name: autodev
        image: autodev-agent:latest
        env:
        - name: OPENAI_API_KEY
          valueFrom:
            secretKeyRef:
              name: autodev-secrets
              key: openai-api-key
        volumeMounts:
        - name: workspace
          mountPath: /workspace
      volumes:
      - name: workspace
        persistentVolumeClaim:
          claimName: workspace-pvc
```