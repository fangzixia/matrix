# Matrix 部署指南

Matrix 为 **B/S Web 应用**，唯一入口为 `cmd/web`。部署方式为 **本机直接运行**（Go 二进制 + 本地/远程 PostgreSQL），不使用 Docker。

## 环境要求

| 组件 | 版本 |
|------|------|
| Go | 1.24+ |
| PostgreSQL | 16+ |
| Node.js | 22+（仅前端构建时需要） |
| Git | 可选（项目 Git 克隆需要） |

## 1. 安装 PostgreSQL

### Windows

1. 从 [PostgreSQL 官网](https://www.postgresql.org/download/windows/) 安装。
2. 打开 **psql** 或 pgAdmin，创建数据库与用户：

```sql
CREATE USER matrix WITH PASSWORD 'matrix';
CREATE DATABASE matrix OWNER matrix;
GRANT ALL PRIVILEGES ON DATABASE matrix TO matrix;
```

### Linux（示例：Debian/Ubuntu）

```bash
sudo apt install postgresql postgresql-contrib
sudo -u postgres psql -c "CREATE USER matrix WITH PASSWORD 'matrix';"
sudo -u postgres psql -c "CREATE DATABASE matrix OWNER matrix;"
```

确保 PostgreSQL 监听 `localhost:5432`（或你计划写入 DSN 的地址）。

## 2. 配置

```bash
cp config/config.example.yml config/config.yml
```

编辑 `config/config.yml`，至少确认以下项：

| 配置 | 说明 |
|------|------|
| `database.dsn` | 例如 `postgres://matrix:matrix@localhost:5432/matrix?sslmode=disable` |
| `auth.bootstrap.admin_password` | 首装 admin 密码；可用环境变量 `MATRIX_BOOTSTRAP_PASSWORD` 覆盖 |
| `storage.data_dir` | 系统数据目录（工作区、审计等） |
| `logging.dir` | 日志目录 |

首装登录后，使用 **root** 在 **管理区域 → 系统配置**（`/admin/system`）设置默认模型 API Key、MCP、Git、Worker 并发与流水线等（不再写入 `config.yml`）。

环境变量占位符支持 `${VAR}` 与 `${VAR:-default}`。

## 3. 构建

### 前端（生产）

```bash
cd frontend
npm install
npm run build
cd ..
```

构建产物输出到 `frontend/dist/`，由 Go embed 打包进 Web 二进制。

### 后端

```bash
# Windows
go build -o build/matrix.exe ./cmd/web

# Linux / macOS
go build -o build/matrix ./cmd/web
```

或使用一键脚本（含前端构建）：

```bash
scripts/build-web.bat    # Windows
scripts/build-web.sh     # Linux / macOS
```

## 4. 启动

```bash
# 开发（源码运行）
go run ./cmd/web -config config/config.yml

# 生产（二进制）
./build/matrix -config config/config.yml
```

Windows PowerShell：

```powershell
.\build\matrix.exe -config config\config.yml
```

可选环境变量：

```bash
set MATRIX_CONFIG=config/config.yml
set MATRIX_BOOTSTRAP_PASSWORD=your-secure-password
```

默认监听 `:8080`。首次启动若 `users` 表为空，会自动创建 admin 用户 `root`。

访问 http://localhost:8080 登录。

## 5. 前端开发模式

后端与前端分开运行，Vite 代理 API：

```bash
# 终端 1：后端
go run ./cmd/web -config config/config.yml

# 终端 2：前端
cd frontend && npm run dev
```

浏览器访问 http://localhost:5173 。

## 6. 生产建议

1. 使用 **systemd**（Linux）或 **Windows 服务 / 任务计划程序** 托管 `matrix` 进程，设置开机自启。
2. 前置 **Nginx / Caddy** 反向代理，启用 HTTPS；将 `auth.session.secure` 设为 `true`。
3. 修改 bootstrap 密码，创建普通用户后禁用或删除默认 admin。
4. 在管理区域 → 系统配置中保持 **允许 Shell** 关闭（默认）。
5. 定期备份 PostgreSQL 与 `storage.data_dir` 目录。

### Linux systemd 示例

`/etc/systemd/system/matrix.service`：

```ini
[Unit]
Description=Matrix Web
After=network.target postgresql.service

[Service]
Type=simple
User=matrix
WorkingDirectory=/opt/matrix
Environment=MATRIX_CONFIG=/opt/matrix/config/config.yml
ExecStart=/opt/matrix/build/matrix
Restart=on-failure

[Install]
WantedBy=multi-user.target
```

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now matrix
```

## 7. 健康检查

```bash
curl http://localhost:8080/health
```

## 8. 异步任务（嵌入式 Worker）

Pipeline 与默认 Run 会写入 PostgreSQL `run_jobs` 队列。**无需单独启动 Worker 进程**——`cmd/web` 启动时会自动在后台消费队列。

`config/config.yml` 中 `worker` 段已移除；队列消费者在 `cmd/web` 内嵌启动，**root** 可在 Web **管理区域 → 系统配置** 调整 `enabled`、`poll_interval`、`max_attempts`、`concurrency` 等。

```yaml
# 流水线默认阶段等亦在系统配置页设置，示例：
# default_stages: [spec, implement, verify, build]
# pull_before_stage: true
```

开发调试可在 API 上加 `?sync=1` 同步执行（不经队列）。

> `cmd/worker` 已弃用：若误启动会打印提示并以退出码 1 结束，请统一使用 `cmd/web`。
