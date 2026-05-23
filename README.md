# Matrix Desktop

Matrix 是一款基于 **Wails v2** 的 AI 驱动桌面开发助手。通过 Coordinator + Worker Agent 架构，在本地工作区内完成对话、任务编排、工具调用与诊断导出，支持 MCP 扩展。

---

## 功能概览

| 模块 | 说明 |
|------|------|
| **自由对话** | 多轮 Chat，续接完整 Agent transcript |
| **Build Agent** | Spec / Implement / Verify / Build / UI Scan 等任务流 |
| **工作区** | 切换项目目录，工具读写限定在工作区内 |
| **MCP** | 可配置本地/远程 MCP 服务器与工具 |
| **诊断** | 会话 JSONL 审计、Markdown 导出，便于粘贴给 LLM 分析 |
| **子 Agent** | Coordinator 派发 Worker，sidechain 持久化 |

---

## 技术栈

- **后端**：Go 1.24+（`internal/`）
- **前端**：原生 HTML / CSS / JavaScript（`frontend/static/`）
- **桌面壳**： [Wails v2](https://wails.io/)
- **LLM**：OpenAI 兼容 API（默认 DeepSeek）

---

## 快速开始

### 环境要求

- Go 1.24+
- [Wails CLI v2](https://wails.io/docs/gettingstarted/installation)
- Windows：WebView2（Win10/11 通常已内置）

### 开发运行

```bash
# 安装依赖
go mod tidy

# 开发模式（热重载）
wails dev
```

### 生产构建

```bash
# Windows
scripts/build-desktop.bat

# 或
wails build
```

产物位于 `build/bin/`。

---

## 项目结构

```
matrix/
├── main.go                 # Wails 入口
├── wails.json
├── frontend/               # 前端静态资源 + wailsjs 绑定
│   ├── index.html
│   ├── static/js/app.js    # 主 UI 逻辑
│   └── embed.go
├── internal/
│   ├── desktop/            # Wails Bridge、配置、Chat、会话编排
│   ├── matrixpaths/        # ★ 统一路径解析（应用数据 + 工作区 ID）
│   ├── audit/              # 会话诊断 JSONL
│   ├── agent/              # 子 Agent 注册与 sidechain
│   ├── coordinator/        # 父 Agent 工具集、StreamHub
│   ├── query/              # TAOR 循环、上下文压缩
│   ├── tools/              # 内置工具（读写文件、grep、todo_write…）
│   ├── llm/                # OpenAI 兼容客户端
│   ├── mcp/                # MCP 连接管理
│   └── logger/             # 结构化运维日志
└── docs/
    └── LOGGING.md          # 日志与诊断说明
```

---

## 架构设计

### Agent 分层

```
┌─────────────────────────────────────────┐
│  Wails Bridge (desktop/)                │
│  RunChatSession / RunTask / SetWorkspace│
└─────────────────┬───────────────────────┘
                  │
┌─────────────────▼───────────────────────┐
│  Coordinator（父 Agent）                 │
│  agent / send_message / task_stop       │
└─────────────────┬───────────────────────┘
                  │ 派发
┌─────────────────▼───────────────────────┐
│  Worker Agent + 工具集                   │
│  read / write / grep / mcp_* / todo…    │
└─────────────────────────────────────────┘
```

- **Coordinator** 负责编排与流式推送（`agent:stream` 事件）。
- **Worker** 持有执行类工具；子 Agent 通过 `agent` 工具嵌套派发。
- **query.RunSession** 实现 TAOR 循环、上下文压缩与 audit 钩子。

### 自由对话：双存储模型

| 层 | 存储位置 | 内容 |
|----|----------|------|
| **对话历史** | `chat-history.json` | 标题、展示消息、执行快照（供界面渲染） |
| **Agent transcript** | `transcripts/{id}.json` | 完整 `query.Message`（含 tool/thinking，供多轮续接） |

前端通过 `GetChatSessions` / `SaveChatSessions` 读写对话历史；发送消息时 `RunChatSession` 使用 `chatSessionId` 续接 Agent transcript。若 transcript 为空，可用对话历史作为 **bootstrap** 降级恢复。

---

## 数据与路径（重要）

**所有运行时 durable 数据均在应用数据目录，不会写入用户项目目录。**

### 应用数据根目录

```
%UserConfigDir%/matrix/          # Windows: %APPDATA%\matrix
```

由 `matrixpaths.AppDataDir()` 解析；配置、日志、各工作区会话数据均在其下。

### 目录布局

```
%UserConfigDir%/matrix/
├── config.json                      # API Key、模型、MCP、最近工作区
├── logs/
│   └── matrix.log                   # 运维日志
└── workspaces/
    └── {workspace_id}/              # workspace_id = sha256(规范化绝对路径)[0:16]
        ├── meta.json                # 工作区路径、最后打开时间
        ├── sessions/                # Agent 审计 JSONL + meta
        ├── transcripts/             # Agent transcript
        ├── chat-history.json        # 对话历史列表
        ├── todos.json               # Agent todo_write 持久化
        ├── subagents/               # 子 Agent sidechain JSONL
        └── exports/                 # 诊断导出默认目录
```

### 工作区 ID

- 输入：用户选择的**项目目录绝对路径**（如 `E:\projects\myapp`）
- `workspace_id` 由路径规范化后 SHA256 取前 16 位 hex，大小写/斜杠不敏感
- 所有 `matrixpaths.SessionsDir(workspacePath)` 等函数的参数均为**项目路径**，返回值在应用数据目录

### 用户项目目录

```
your-project/
├── src/ …                           # 你的代码（Agent 工具可读写）
└── .spec/                           # 需求/评测等业务产物（Matrix 任务流）
```

路径 API 统一在 `internal/matrixpaths/`；**不要**在业务代码中硬编码 `%APPDATA%`。

---

## 配置

配置文件：`%UserConfigDir%/matrix/config.json`

### 主要字段

```json
{
  "model": {
    "baseUrl": "https://api.deepseek.com",
    "apiKey": "YOUR_API_KEY",
    "model": "deepseek-reasoner",
    "maxTokens": 8192,
    "smartCompressThreshold": 100000
  },
  "workspace": {
    "root": "E:\\\\path\\\\to\\\\project",
    "recent": ["…"]
  },
  "context": {
    "microCompactThreshold": 100000,
    "keepRecentToolResults": 3,
    "contextLimitTokens": 196608,
    "autoCompactThreshold": 100000,
    "keepRecentMessages": 8
  },
  "mcpServers": {
    "my-server": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-filesystem", "E:\\\\data"],
      "disabled": false,
      "autoApprove": ["read_file"]
    }
  }
}
```

| 字段 | 说明 |
|------|------|
| `model.apiKey` | **必填**，否则对话/任务会提示「请先配置 API Key」 |
| `model.baseUrl` | OpenAI 兼容 API 根地址 |
| `workspace.root` | 当前工作区绝对路径 |
| `mcpServers` | MCP 服务器定义；支持 `command`（本地）或 `url`（远程） |

也可在应用内 **配置** 页修改并热重载。

---

## 日志与诊断

详见 [docs/LOGGING.md](docs/LOGGING.md)。

| 类型 | 路径 |
|------|------|
| 运维日志 | `%UserConfigDir%/matrix/logs/matrix.log` |
| 会话审计 | `…/workspaces/{id}/sessions/{sessionId}.jsonl` |

开发模式（`wails dev` / `matrix-dev.exe`）：Text 日志 + stderr 双写 + Debug 级别。

环境变量：

- `MATRIX_DEV=1` — 强制开发模式
- `MATRIX_LOG_FORMAT=json|text` — 日志格式

---

## 主要 API（Wails Bridge）

| 方法 | 说明 |
|------|------|
| `GetWorkspace` / `SetWorkspace` | 工作区；返回 `current`、`workspaceId`、`recent` |
| `GetSettings` / `SaveSettings` | 读写配置 |
| `RunChatSession` | 自由对话多轮 |
| `GetChatSessions` / `SaveChatSessions` | 对话历史列表 |
| `ClearChatSession` | 清除 Agent transcript |
| `RunSpec` / `RunImplement` / … | 任务流 |
| `ListSessionDiagnostics` | 审计会话列表 |
| `GetSessionDiagnostic` | 导出 Markdown 诊断包 |
| `ListSubAgents` / `StopSubAgent` | 子 Agent 管理 |
| `TestMCPServer` / `CallMCPTool` | MCP 操作 |

前端通过 `frontend/static/js/api-adapter.js` 封装调用。

---

## 开发说明

### 修改 Go 绑定后

```bash
wails generate module
```

绑定生成物在 `frontend/wailsjs/`（已 `.gitignore`，本地生成）。

### 测试

```bash
go test ./...
```

路径相关测试使用 `matrixpaths.SetDataRootForTest` 重定向到临时目录。

### 代码约定

- **路径**：仅通过 `matrixpaths` 包；参数 `workspacePath` = 用户项目绝对路径
- **工作区工具边界**：`tools.SetWorkspaceRoot` 在 `SetWorkspace` 与 `runAgentSession` 入口设置
- **Chat**：`ChatTranscriptStore`（Agent transcript）与 `ChatSessionStore`（对话历史）职责分离

---

## 许可证

Copyright © 2026 Matrix. 见 `wails.json` 与仓库说明。
