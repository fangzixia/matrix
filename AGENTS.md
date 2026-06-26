<!-- CODEGRAPH_START -->
## CodeGraph

In repositories indexed by CodeGraph (a `.codegraph/` directory exists at the repo root), reach for it BEFORE grep/find or reading files when you need to understand or locate code:

- **MCP tool** (when available): `codegraph_explore` answers most code questions in one call — the relevant symbols' verbatim source plus the call paths between them, including dynamic-dispatch hops grep can't follow. Name a file or symbol in the query to read its current line-numbered source. If it's listed but deferred, load it by name via tool search.
- **Shell** (always works): `codegraph explore "<symbol names or question>"` prints the same output.

If there is no `.codegraph/` directory, skip CodeGraph entirely — indexing is the user's decision.
<!-- CODEGRAPH_END -->

## 运行时日志

Matrix 后端将系统运行日志写入 **`logs/matrix.log`**（路径由 `config.yml` 的 `logging.dir` / `logging.file` 决定；轮转备份为 `matrix.log.1`、`matrix.log.2` …）。

**保留策略：** 日志目录已在 `.gitignore` 中排除（不提交仓库），但**本地磁盘上的日志必须保留**，用于事后排查。不要主动删除 `logs/` 下的文件，除非用户明确要求清理。

**排查问题时：**

1. 先读 **`logs/matrix.log`** 末尾数百行，或按时间 / 关键字检索（`request_id`、`session_id`、`run:`、`loop:`、`level=ERROR`）。
2. 关键字段：
   - `request_id` — 对应单次 HTTP 请求
   - `session_id` / `runId` — 对应 AI Run / 对话会话
   - `component=query|coordinator` — AI 内核组件
   - `loop: llm request|response|tool execution|tool result` — Agent 每轮 TAOR
   - `run: build query config` — Run 启动
3. 若当前文件已轮转，继续查看 `matrix.log.1` 等备份。
4. 结合前端报错时间、Run ID、项目 ID 在日志中定位同一时段记录，再追溯根因。

配置项（见 `config/config.example.yml`）：

| 项 | 说明 |
|----|------|
| `logging.level` | `debug` 保留最完整信息；生产可用 `info` |
| `logging.max_size_mb` | 单文件上限（MB），超出后轮转 |
| `logging.max_backups` | 保留的轮转备份份数；排查需要更长历史时可增大 |

## 问题修复原则

修复 bug 或实现需求时，**先分析根因，再从根本上解决**，不要打补丁。

**根因分析：**

1. 复现问题，确认现象与触发条件（输入、状态、时序）。
2. 追溯调用链与数据流，定位**最早出错或设计缺陷**的位置，而非仅修复报错栈顶。
3. 结合 `logs/matrix.log`、测试、代码阅读，区分「表面症状」与「底层原因」。
4. 若存在多个相关症状，找出共同根因，避免逐个打补丁。

**根本性修复（优先）：**

- 修正错误的设计、状态管理、边界条件或契约不一致。
- 让正确路径自然成立，而不是在错误路径上叠加 `if`、重试、吞异常、默认值兜底。
- 删除或重构导致问题的冗余/过时逻辑，而非在其旁边再叠一层 workaround。
- 改动范围聚焦根因，但允许为消除整类问题做必要的小范围重构。

**避免打补丁：**

- 不为了「先让它跑起来」而忽略未理解的失败原因。
- 不在多处重复相同的防御性代码来掩盖同一根因。
- 不用 magic number、硬编码分支、注释 `TODO` 代替真正修复。
- 若临时方案不可避免，必须说明根因、为何无法立即根治、以及后续应如何收尾。
