# Matrix UI 规范

前端采用 **Vue 3 + Vite + Pinia + vue-router**，视觉参考 GitLab 15+ 浅色主题（Pajamas-inspired）。

## 设计令牌

定义于 `frontend/src/assets/styles/tokens.scss`：

- 主色：`--gl-color-orange-500` / `--gl-color-orange-600`
- 背景：`--gl-background-color-default` / `--gl-background-color-subtle`
- 边框：`--gl-border-color-default`
- 布局：`--gl-sidebar-width: 220px`、`--gl-header-height: 48px`

## 布局

| 布局 | 路径 | 说明 |
|------|------|------|
| `AuthLayout` | `/users/sign_in` | 居中登录卡片 |
| `AppShell` | 主应用 | 顶栏 + 项目侧栏 + 内容区 |
| `AdminLayout` | `/admin/*` | 管理区独立侧栏 |
| `ProjectLayout` | `/projects/:id/*` | 项目标题 + 子路由 |

## 通用组件

位于 `frontend/src/components/ui/`：

- `GlButton` — primary / secondary / danger / link
- `GlTable` — 数据表格
- `GlAlert` — 提示条
- `GlBadge` — 状态徽章
- `GlModal` — 确认对话框
- `GlTabs` — 设置页 Tab 导航
- `MemberRoleSelect` — Guest → Owner 五级角色

## 路由

完整路由见 `frontend/src/router/index.ts`。守卫逻辑：

- 未登录 → `/users/sign_in?redirect=...`
- 非 admin 访问 `/admin` → `/403`

## API 客户端

- `src/api/client.ts` — fetch 封装，`credentials: 'include'`
- `src/api/stream.ts` — Run SSE `EventSource`

开发模式 Vite 代理 `/api` 至 `localhost:8080`；生产由 Go embed `frontend/dist` 同域托管。

## 构建

```bash
cd frontend
npm install
npm run build   # 输出到 frontend/dist/
go build ./cmd/web
```
