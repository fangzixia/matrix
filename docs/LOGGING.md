# Matrix 日志与诊断

## 运维日志（matrix.log）

- 路径：`%UserConfigDir%/matrix/logs/matrix.log`（由 `matrixpaths.LogFile()` 解析）
- 开发模式（`MATRIX_DEV=1` 或 `matrix-dev` 可执行文件）：Text 格式 + stderr 双写 + Debug 级别
- 生产默认：JSON Lines（便于 `jq` 与大模型解析）
- 覆盖格式：`MATRIX_LOG_FORMAT=text` 或 `json`

结构化字段（通过 `logger.With` / `logger.InfoCtx`）：

- `session_id`, `agent_id`, `turn`, `component`

## 会话诊断（面向 LLM）

每个 Agent 会话在**应用数据目录**（非项目工作区）写入：

- `%UserConfigDir%/matrix/workspaces/{workspace_id}/sessions/{sessionID}.jsonl` — 事件时间线
- `%UserConfigDir%/matrix/workspaces/{workspace_id}/sessions/{sessionID}.meta.json` — 摘要元数据

子 Agent 详细流：`.../subagents/{agentId}.jsonl`。

### 导出方式

1. **Bridge API**（Wails）  
   - `ListSessionDiagnostics(limit)`  
   - `GetSessionDiagnostic(sessionID)` → `llm_markdown` 可直接粘贴给大模型  
   - `ExportSessionDiagnosticToFile(sessionID, destDir)`

2. **直接读文件**  
   打开上述 jsonl / meta，或使用导出目录中的 `*-diagnostic.md`。

### 典型事件

| event | 含义 |
|-------|------|
| `session.start` / `session.end` | 会话起止 |
| `turn.iteration` | TAOR 轮次 |
| `turn.llm_request` / `turn.llm_response` | 模型调用 |
| `turn.tool_call` / `turn.tool_result` | 工具执行 |
| `context.compact` | 上下文压缩 |
| `async.result` | 异步子 Agent 结果注入 |
| `subagent.spawn` / `subagent.done` | 子 Agent 生命周期 |

敏感信息（API Key、Bearer 等）在写入前经 `audit.RedactString` 处理。
