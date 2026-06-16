# Matrix 日志与诊断

## 系统日志（slog）

Matrix Web 使用 Go 标准库 **slog** 输出运维日志，由 YAML `logging` 段配置：

```yaml
logging:
  dir: ./logs              # 日志目录（可与 storage.data_dir 分离）
  file: matrix.log         # 主日志文件名
  level: info              # debug | info | warn | error
  format: json             # json | text（development 默认 text）
  max_size_mb: 100         # 单文件上限，超出后轮转
  max_backups: 7           # 保留归档数量
```

解析后写入 `{logging.dir}/{logging.file}`。开发模式（`system.env: development`）同时输出到 stderr。

### 日志内容

| 类型 | 说明 |
|------|------|
| HTTP 访问 | method、path、status、latency、request_id |
| 启动 | 存储路径解析、监听地址、迁移结果 |
| Run 状态 | Run 创建/完成/取消 |
| Git 操作 | clone/pull/push 错误 |

### 环境变量

| 变量 | 说明 |
|------|------|
| `MATRIX_CONFIG` | 配置文件路径 |
| `MATRIX_DEV=1` | 可配合开发模式使用 |

## AI 审计（JSONL）

Agent 会话审计 **不走 slog**，由 `internal/ai/audit` 写入系统目录：

```
{storage.audit_dir}/{project_id}/{run_id}.jsonl
{storage.audit_dir}/{project_id}/sessions/
```

路径由 `platform/storage.Paths` 在启动时解析；`runs.audit_path` 字段在 PG 中索引对应文件。

## 与旧版桌面版的区别

| 旧版（Wails） | 新版（Web） |
|---------------|-------------|
| `%APPDATA%/matrix/logs/matrix.log` | YAML `logging.dir` 可配 |
| `matrixpaths` 包 | `platform/storage` |
| 自定义 logger | slog |

## 排查建议

1. 启动失败：查看控制台或 `logging.dir` 下最新日志。
2. Run 无输出：检查 SSE 连接与 `{audit_dir}` 下 JSONL 是否生成。
3. 迁移失败：日志中会记录 `migrate up` 错误；确认 PostgreSQL 可连且 `migrations/` 目录可读。
