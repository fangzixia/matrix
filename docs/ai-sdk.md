# Matrix 与 AI SDK

Matrix 将 [`matrix/ai`](../ai/README.md) 作为独立 Go module 引用（根 `go.mod`：`replace matrix/ai => ./ai`）。

**宿主只需 `import "matrix/ai/sdk"`**（可用 `ai "matrix/ai/sdk"`），无需直接依赖 `query` / `stream` 等子包。

## SDK 主文档

完整接入指南、Stable API、事件契约与 ID 模型见 **[`ai/README.md`](../ai/README.md)**。

最小可运行示例：**[`ai/examples/minimal`](../ai/examples/minimal/main.go)**（单一 `import "matrix/ai/sdk"`）。

## Matrix 宿主适配层

| 路径 | 职责 |
|------|------|
| `internal/modules/run/agent_session.go` | 组装 `query.Config`、MCP、`coordinator`、审计 Writer |
| `internal/modules/run/execute.go` | Job 生命周期、`WithRuntimeDir`、多阶段 `RunID` |
| `internal/modules/run/harness/` | 产品提示词、`FormatWorkspaceUserMessage` |
| `internal/modules/run/view/` | `stream.Sink` 实现、AG-UI → UI 投影、SSE |
| `internal/modules/run/view/activity/` | UI 进度文案（原 `ai/activity`） |
| `internal/modules/run/audit/` | 会话 JSONL 诊断（原 `ai/audit`） |
| `internal/app/bootstrap/bootstrap.go` | `llm.SetHTTPLogHooks` → `llm.log` |

宿主在 `execute.attachRunCancel` 经 `ai.WithPolicy` 注入文件访问策略（可读写根目录 + ScratchDir）。

## ID 与事件

- **Job ID**（`runs.id`）= 宿主 `jobId`，写入 `run_view_events.job_id`
- **AG-UI `runId`** = 每次 `RunSession` 唯一；Build 多阶段每阶段 `stream.NewRunID()`
- **`threadId`** = `projectId`（或 chat session）
- Job 终态由宿主发 `CUSTOM` / `job_run_finished`（非 SDK 原生 `RUN_FINISHED`）

详见 [`docs/UI.md`](UI.md) 与 [`ai/README.md`](../ai/README.md#两层-id-模型)。
