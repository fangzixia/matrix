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
| `components/admin/system/*Tab.tsx` | 管理后台业务配置页（非通用 UI） |

## 设计令牌

定义于 `frontend/src/assets/styles/tokens.scss`：

- 主色：`--matrix-color-orange-500` / `--matrix-color-orange-600`
- 背景：`--matrix-background-color-default` / `--matrix-background-color-subtle`
- 边框：`--matrix-border-color-default`
- 布局：`--matrix-sidebar-width: 240px`、`--matrix-header-height: 40px`

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

Ant Design 主题配置：`frontend/src/theme/antdTheme.ts`，在 `App.tsx` 通过 `XProvider` 注入。
