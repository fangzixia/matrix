# matrix/ai — Agent Runtime SDK

`matrix/ai` 是可被任意宿主接入的 **TAOR Agent 运行时 SDK**：接收 `Config` 与消息历史，经 `Sink` 输出 **AG-UI 事件流**，并返回同步 `Result`。Matrix 仓库仅作为参考宿主。

## 单一入口（`matrix/ai/sdk`）

宿主**只需**：

```go
import "matrix/ai/sdk"
```

子包（`query` / `stream` / `llm` / `util` / `tools` / `access` / `coordinator` / `agent` / `mcp`）为 SDK 内部实现；公开接入以 `sdk` 包导出符号为准。

## 定位与非目标

| 层级 | 门面符号 | 说明 |
|------|----------|------|
| **Stable 核心** | `RunSession`、`Config`、`Sink`、`Client`、`Policy`、`DefaultRegistry` | 最小可运行面；工具框架在 `util`，内置工具在 `tools` |
| **扩展（Opt-in）** | `CoordinatorConfig`、`StreamHub`、`MCPManager`、`RegisterMCPTools` | 多 Agent / MCP |
| **宿主自有** | — | UI 文案、审计 JSONL、SSE/DB、产品提示词 |

## 安装

同仓（`go.mod` 已 `replace`）：

```go
import "matrix/ai/sdk"
```

独立引用时在宿主 `go.mod` 增加：

```go
require matrix/ai v0.0.0
replace matrix/ai => ../ai
```

## 最小接入

完整示例见 [`examples/minimal`](examples/minimal/main.go)（**仅** `import "matrix/ai/sdk"`）。

```go
result := sdk.RunSession(ctx, sdk.Config{
    LLM:      client,
    Model:    "your-model",
    ThreadID: "project-or-chat-id", // 必填
    Registry: sdk.RegistryWithoutShell(nil),
    InitialMessages: []sdk.Message{
        {Role: sdk.RoleUser, Content: "Hello"},
    },
}, sink)
```

`RunSession` 入口会调用 `Config.Validate()`；`RunID` 为空时自动生成。

## `Config` 字段

| 字段 | 必填 | 说明 |
|------|------|------|
| `LLM` | 是 | OpenAI 兼容客户端（`sdk.Client`） |
| `Model` | 是 | API 模型名 |
| `ThreadID` | 是 | AG-UI `threadId`（会话维度） |
| `RunID` | 否 | AG-UI `runId`；空则 SDK 生成 |
| `ParentRunID` | 否 | 谱系 ID（如宿主 Job ID） |
| `SessionID` | 否 | 日志/审计关联；默认 `RunID` |
| `Registry` | 否 | `sdk.ToolRegistry` |
| `Audit` | 否 | `sdk.AuditRecorder` 可选诊断接口 |

## Sink 与 AG-UI 事件

`RunSession` 经 `sdk.Sink` 发出：`RUN_STARTED` → `STEP_*` / `TEXT_*` / `TOOL_CALL_*` → `RUN_FINISHED` | `RUN_ERROR`。

常用：`sdk.FuncSink`、`sdk.NewCoalesceSink`、`sdk.BuildCoalescedSink`、`sdk.NewRunID`、`sdk.RunStarted`、`sdk.EventType`。

## 文件访问策略

沙箱由宿主定义；文件类工具只校验路径是否在 `sdk.Policy` 允许范围内：

```go
ctx = sdk.WithPolicy(ctx, sdk.NewPolicy(
    []string{absProjectRoot, absDocsRoot},
    absRunScratchDir,
))
```

## 扩展：Coordinator 与 MCP

```go
coordCfg := sdk.CoordinatorConfig{ /* ... */ }
hub := sdk.NewStreamHub(threadID, runID, registry, sidechain, sink, nil, onUpdate, onDone)
_ = sdk.RegisterMCPTools(reg, mcpMgr)
```

## 日志

SDK 使用 `log/slog`。LLM HTTP 旁路：`sdk.SetHTTPLogHooks`。

## 两层 ID 模型

| 字段 | 含义 | 生成方 |
|------|------|--------|
| `ThreadID` | 会话维度 | **宿主，必填** |
| `RunID` | 单次 `RunSession` | 宿主或 SDK |
| `ParentRunID` | 谱系（如 Job ID） | 宿主 |
| `SessionID` | 审计/日志 | 默认 `RunID` |

## 门面 API 索引

### 会话
`RunSession` `Config` `Message` `Result` `Role` `StopReason` `ContextPolicy` `AuditRecorder` `PreviewText`

### 事件流
`Sink` `Event` `FuncSink` `NopSink` `NewRunID` `BuildCoalescedSink` + emit helpers

### LLM
`NewClient` `Client` `SetHTTPLogHooks` `HTTPMeta`

### 访问策略
`Policy` `WithPolicy` `NewPolicy` `ResolveAllowed`

### 工具
`DefaultRegistry` `RegistryWithoutShell` `ToolRegistry` `Tool`

工具执行框架（`util.RunTools`）负责 `TOOL_CALL_START` / `TOOL_CALL_END` 等 AG-UI 生命周期事件；工具实现只写流式内容：

- 可选 `Tool.StatusLabel(args)` 声明开始文案（`execOne` 在调用 `Execute` 前写入一行）
- 中途/结果预览：`util.StreamWriter(ctx)`（`io.Writer`）；无 Sink 时为 `io.Discard`
- **禁止**在工具内直接推送 AG-UI 事件

自定义工具示例：

```go
&util.Tool{
    Name: "my_tool",
    StatusLabel: func(args map[string]any) string { return "处理中 …" },
    Execute: func(ctx context.Context, args map[string]any) (string, error) {
        io.WriteString(util.StreamWriter(ctx), "chunk\n")
        return "done", nil
    },
}
```

### Extension
`CoordinatorConfig` `StreamHub` `AsyncSupport` `RunControl` `AgentRegistry` `AgentSnapshot` `MCPManager` `RegisterMCPTools`

## Breaking：公开接入路径

宿主应使用 `import "matrix/ai/sdk"`（可用别名 `ai "matrix/ai/sdk"`）。子包路径仍存在于 module 内供高级定制，但**不再作为公开接入方式**文档化。

| 旧 import | 门面 |
|-----------|------|
| `matrix/ai`（根包） / `matrix/ai/api` | `matrix/ai/sdk` |
| `matrix/ai/query` | `sdk.Config` `sdk.RunSession` … |
| `matrix/ai/stream` | `sdk.Sink` `sdk.Event` … |
| `matrix/ai/coordinator` | `sdk.CoordinatorConfig` `sdk.StreamHub` … |
| `matrix/ai/agent` | `sdk.AgentRegistry` `sdk.AgentSnapshot` … |
| `matrix/ai/mcp` | `sdk.MCPManager` `sdk.RegisterMCPTools` |
