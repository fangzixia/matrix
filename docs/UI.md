# Matrix UI 规范

前端采用 **React 19 + Vite + Ant Design 6 + @ant-design/x + Zustand + React Router**。

## 最高优先级（与 README 一致）

1. **只允许** Ant Design 6、@ant-design/x 及其**二次封装**。
2. **禁止** 手写基础 HTML UI 或引入其他 UI 库。
3. **禁止** 为 Ant Design 已提供的组件场景再建包装组件。
4. 逻辑复用优先 **hooks** / **utils**，而非 UI 组件。

### 允许的二次封装

| 路径 | 原因 |
|------|------|
| `components/MatrixLogo.tsx` | Ant Design 无品牌 Logo |
| `components/ai/MatrixAiChat.tsx` | AI 对话，基于 @ant-design/x |
| `components/ai/ProjectChatWorkspace.tsx` | 会话列表 + 对话主区拼装（Conversations + MatrixAiChat） |
| `components/ai/PlanComposePanel.tsx` | 计划编写场景对话壳 |
| `components/ai/RunActivityPanel.tsx` | Run 过程可视化（ThoughtChain / Collapse） |
| `components/admin/system/*Tab.tsx` | 管理后台业务配置页（非通用 UI） |
| `components/docs/*` | Markdown / 文档选择等业务薄封装 |

## 设计令牌

定义于：

| 文件 | 内容 |
|------|------|
| `frontend/src/theme/colors.ts` | 主色、背景、边框、Logo 印记色、阴影 |
| `frontend/src/theme/antd-theme.ts` | Ant Design `ThemeConfig`（经 `XProvider` 注入） |
| `frontend/src/theme/layout.ts` | 侧栏宽度、活动面板最大高度等布局尺寸 |
| `frontend/src/theme/surface.ts` | 页面 Card 表面样式辅助 |

布局常量：

- App 侧栏：`MATRIX_LAYOUT.appSiderWidth`（220）
- 对话会话列表：`MATRIX_LAYOUT.chatSiderWidth`（240）
- 活动面板最大高度：`MATRIX_LAYOUT.activityPanelMaxHeight`（280）

颜色与间距在组件内优先使用 `theme.useToken()`；禁止在页面/组件中硬编码 `#rgb` / `rgba(...)`（色板源文件除外）。

## 布局

| 布局 | 路径 | 说明 |
|------|------|------|
| `AuthLayout` | `/users/sign_in` | 居中登录卡片 |
| `AppShell` | 主应用 | 顶栏 + 项目侧栏 + 内容区 |
| `AdminLayout` | `/admin/*` | 管理区独立侧栏 |
| `ProjectLayout` | `/projects/:id/*` | 项目标题 + 子路由 |

## 常用 Ant Design 映射

| 场景 | 组件 |
|------|------|
| 用户头像 | `Avatar` + `avatarInitials()`（`utils/avatar.ts`） |
| 用户搜索 | `AutoComplete` + `useUserSearch()` |
| 成员角色 | `Select` + `memberRoleOptions`（`api/projects.ts`） |
| 项目可见性 | `Tag` + `visibilityLabels` |
| 设置子导航 | `Tabs` + `settingsTabs()` + `useSettingsTabNavigate()` |
| 导航图标 | `@ant-design/icons` |

## 路由

完整路由见 `frontend/src/router/index.tsx`。守卫逻辑：

- 未登录 → `/users/sign_in?redirect=...`
- 非 admin 访问 `/admin` → `/403`
- 非 root 访问 `/admin/system` → `/403`

## 主题

Ant Design 主题配置：`frontend/src/theme/antd-theme.ts`，在 `App.tsx` 通过 `XProvider` 注入。色板源：`frontend/src/theme/colors.ts`。

## Run 视图流（AG-UI 对齐）

### 两层 ID 模型

| 概念 | 字段 | 说明 |
|------|------|------|
| **Matrix Job** | `jobId`（= `runs.id`） | 用户创建的任务；SSE 日志 `run_view_events.job_id`、REST/SSE URL 中的 `runId` 均指此 ID |
| **AG-UI Run** | `event.runId` | 每次 `RunSession` 生成的会话 ID（Build 多阶段每阶段一个新 runId）；仅出现在 AG-UI 事件 JSON 内 |
| **Thread** | `event.threadId` | 对话线程，Matrix 中等于 `projectId`（或 chat session） |

AI 模块只输出标准 AG-UI 扁平事件；Matrix 宿主投影为 `RunViewState`，持久化事件日志，并在 Job 终态时额外发出 `CUSTOM` / `job_run_finished`（非 AG-UI 原生 `RUN_FINISHED`）。

### API

- **SSE**：`GET /api/projects/:id/runs/:runId/stream?mode=chat|detail`，事件名 `run:view`，载荷为 `LoggedEvent`：

```json
{
  "jobId": "<matrix-job-uuid>",
  "seq": 42,
  "timestamp": 1710000000000,
  "event": { "type": "STATE_SNAPSHOT", "snapshot": { ... } }
}
```

- **快照**：`GET /api/projects/:id/runs/:runId/view` → `{ state: RunViewState | null }`
- **工具日志**：`GET /api/projects/:id/runs/:runId/tools/:toolUseId/log`
- **chat 通道**：`RUN_STARTED` + `TEXT_MESSAGE_CONTENT` + `ACTIVITY_SNAPSHOT` + `STATE_SNAPSHOT` + Job 终态 `job_run_finished`
- **detail 通道**：完整 AG-UI 事件日志（含 TOOL/STEP/REASONING 等）
- **SSE 连接时**：服务端从 `run_view_events` 按 seq catch-up；已结束的 Job 补发 `job_run_finished` 后关闭连接；运行中每 2s 轮询新事件
- 前端类型：`frontend/src/types/runView.ts`（`LoggedEvent` + `AguiStreamEvent`）；状态归约：`frontend/src/utils/viewReducer.ts`（`applyAguiEvent`）
