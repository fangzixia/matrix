# Matrix 架构说明

## 产品定位

Matrix 从桌面 AI 助手升级为 **「业务平台 + AI 内核」** 的私有化 B/S Web 平台：

- **业务平台**（`internal/modules/` + `internal/webapp/`）：用户、项目、Git 沙箱、Run/Chat、RBAC
- **AI 内核**（`internal/ai/`）：纯运行时，无 HTTP/DB/User 依赖
- **横向基础设施**（`internal/platform/`）：配置、日志、PG、迁移、SSE、鉴权

## 五层目录

```
cmd/web              HTTP 入口
cmd/worker           可选异步 Runner 骨架
internal/ai/         AI 内核（query/agent/tools/llm/mcp/audit/stream/harness/ports）
internal/modules/    领域模块（identity/iam/project/workspace/run/requirement/artifact）
internal/platform/   基础设施（config/logging/db/migrate/storage/http/auth/events）
internal/app/        依赖装配与 bootstrap 启动链
internal/webapp/     Gin 路由与 Handler 适配
frontend/            React + Vite + Ant Design 6 GitLab 风格 UI
```

## 依赖方向

```
webapp → modules → platform
modules/run → ai/ports（AgentRuntime 接口）
modules → ai/*（通过 Runtime 调用，不反向依赖）
```

AI 内核通过 `internal/ai/ports/runtime.go` 定义 `AgentRuntime`、`RunRequest`、`SecurityPolicy` 等 DTO，由 `modules/run` 实现并编排。

## 存储双轨

| 类型 | 存储 | 配置 |
|------|------|------|
| 业务数据 | PostgreSQL | `database.dsn` |
| 系统文件 | 本地目录 | `storage.data_dir`、`workspaces_dir`、`audit_dir` |
| 日志 | 本地目录 | `logging.dir` |

Git 工作区克隆到 `{workspaces_dir}/{project_id}/`。

## 认证与授权

- Session Cookie：`_matrix_session`（`platform/auth`）
- 用户/会话：`modules/identity`
- 项目 RBAC 五级角色：`modules/iam` + `RequireProject` 中间件
- Admin Area：`RequireAdmin` 中间件

## 事件流

Run/Chat 执行时，`modules/run` 通过 `platform/events.Hub` 发布 SSE 事件；前端 `EventSource` 订阅 `/api/projects/:id/runs/:runId/stream`。

## 安全策略

YAML `ai.security`：

- `allow_shell: false`（默认）— 使用 `tools.RegistryWithoutShell`
- `allow_command_mcp: false` — 限制 command 型 MCP

详见 `internal/modules/run/runtime.go`。
