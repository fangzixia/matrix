# Matrix

> **前端 UI 规范（最高优先级）**  
> Matrix 前端 **只允许** 使用 [Ant Design 6](https://ant.design/) 与 [@ant-design/x](https://x.ant.design/) 提供的组件，以及基于上述组件库的**二次封装**。  
> - **禁止** 手写基础 UI（`<button>`、`<input>`、`<table>` 等）或引入其他 UI 库。  
> - **禁止** 为可用 Ant Design 直接实现的场景再建自定义组件（如 Avatar、Select、Tabs、Dropdown 等）。  
> - **允许** 的二次封装仅限 Ant Design **未提供** 的能力，例如：  
>   - `components/MatrixLogo.tsx` — 品牌 Logo（Ant Design 无 Logo 组件）  
>   - `components/ai/MatrixAiChat.tsx` — AI 对话（基于 @ant-design/x）  
>   - `components/admin/system/*` — 管理后台各配置 Tab 页面（业务模块，非通用 UI）  
> - 业务逻辑复用请优先使用 **hooks**（如 `useUserSearch`）与 **utils**，而非新建 UI 组件。  
> - 图标统一使用 `@ant-design/icons`。  
> 详细约定见 [docs/UI.md](docs/UI.md) 与 `.cursor/rules/frontend-ui.mdc`。

Matrix 是一款 **私有化 B/S Web 平台**，内置 AI Agent 内核，用于在 Git 工作区内完成对话、任务编排、需求文档与代码实现。

---

## 功能概览

| 模块 | 说明 |
|------|------|
| **用户与权限** | Session 登录、Admin 用户管理、项目级 RBAC（Guest → Owner） |
| **项目管理** | 项目 CRUD、成员管理、Git 仓库绑定 |
| **AI Chat / Run** | 自由对话与任务 Run，SSE 流式输出 |
| **Git 工作区** | 按项目克隆/拉取，文件树浏览 |
| **Harness 任务流** | Spec / Implement / Verify / Build |
| **MCP** | Web 管理端配置 MCP 服务器（默认禁用 command MCP） |

---

## 技术栈

- **后端**：Go 1.24、Gin、GORM、PostgreSQL、slog
- **前端**：React 19、Vite、Ant Design 6、@ant-design/x、Zustand、React Router
- **AI 内核**：Coordinator + Worker Agent、OpenAI 兼容 API

---

## 快速开始

### 环境要求

- Go 1.24+
- PostgreSQL 16+（本机安装）
- Node.js 22+（前端开发 / 构建）

### 首次准备

```bash
# 1. 准备 PostgreSQL（创建 matrix 用户与数据库，见 docs/DEPLOY.md）

# 2. 配置
cp config/config.example.yml config/config.yml
# 编辑 database.dsn；模型、MCP 等请在 Web 管理端 /admin/system 配置

# 3. 安装前端依赖（日常开发只需一次）
cd frontend && npm install && cd ..
```

默认 admin：`root` / `changeme`（可通过 `MATRIX_BOOTSTRAP_PASSWORD` 覆盖）。

### 日常开发（推荐）

生产构建会把 `frontend/dist` 通过 `go:embed` 打进 Go 二进制，**每次改前端都要重新 `npm run build` + `go build` + 重启服务**。日常改 UI 请用 **Vite 开发服务器**：前后端分进程运行，前端支持 HMR，改完即生效，无需打包或重启后端。

**终端 1 — 后端 API（`:8080`）**

```bash
go run ./cmd/web -config config/config.yml
```

**终端 2 — 前端开发服（`:5173`）**

```bash
cd frontend && npm run dev
```

浏览器访问 **http://localhost:5173**（不要访问 `:8080`，否则仍是嵌入的旧构建产物）。

Vite 会把 `/api` 代理到 `http://localhost:8080`（见 `frontend/vite.config.ts`），Cookie / Session 与单进程模式一致。

| 变更类型 | 需要做什么 |
|----------|------------|
| 改 React / 样式 / 路由 | 保存即可，浏览器自动热更新 |
| 改 Go 后端 / 配置 | 重启终端 1 的 `go run` |
| 发布生产包 | 见下方「生产构建」 |

### 单进程启动（快速体验）

不跑 Vite、只用一个端口时，需先构建前端再启动 Go（适合验收 embed 产物，不适合日常改 UI）：

```bash
cd frontend && npm run build && cd ..
go run ./cmd/web -config config/config.yml
```

访问 http://localhost:8080 。

### 生产构建

先编译前端再嵌入 Go 二进制：

```bash
# 推荐：一键脚本
scripts/build-web.bat          # Windows
scripts/build-web.sh           # Linux/macOS

# 或手动两步（go build 不会自动跑 npm）
go generate ./frontend/...
go build -o build/matrix.exe ./cmd/web    # Windows
go build -o build/matrix ./cmd/web        # Linux/macOS
```

已有 `frontend/dist` 且无需重编前端时，可跳过：`set SKIP_FRONTEND=1` 后执行 `go generate`。

完整部署说明见 [docs/DEPLOY.md](docs/DEPLOY.md)。

---

## 项目结构

```
matrix/
├── cmd/web/                 # HTTP 服务入口
├── cmd/worker/              # 异步 Runner 骨架（可选）
├── config/config.example.yml
├── frontend/                # React + Vite + Ant Design 前端
├── internal/
│   ├── ai/                  # AI 内核
│   ├── modules/             # 业务模块
│   ├── platform/            # 基础设施
│   ├── app/                 # 装配与 bootstrap
│   └── webapp/              # HTTP 适配
└── docs/
    ├── DEPLOY.md
    ├── ARCHITECTURE.md
    └── UI.md
```

架构说明见 [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md)。

---

## 配置要点

`config/config.yml`（示例见 `config/config.example.yml`）：

| 段 | 说明 |
|----|------|
| `storage` | 系统数据、工作区、审计、日志目录 |
| `database` | PostgreSQL DSN，启动时 AutoMigrate |
| `auth` | Session Cookie、Bootstrap admin |

模型、MCP、Git、Worker 并发、流水线等运行参数由 **root** 在 Web **管理区域 → 系统配置**（`/admin/system`）中设置，持久化到数据库，不再写入 `config.yml`。

环境变量支持 `${VAR}` 与 `${VAR:-default}` 展开。

---

## 测试

```bash
go test ./...
```

---

## 许可证

Copyright © 2026 Matrix.
