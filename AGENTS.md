<!-- CODEGRAPH_START -->
## CodeGraph

In repositories indexed by CodeGraph (a `.codegraph/` directory exists at the repo root), reach for it BEFORE grep/find or reading files when you need to understand or locate code:

- **MCP tool** (when available): `codegraph_explore` answers most code questions in one call — the relevant symbols' verbatim source plus the call paths between them, including dynamic-dispatch hops grep can't follow. Name a file or symbol in the query to read its current line-numbered source. If it's listed but deferred, load it by name via tool search.
- **Shell** (always works): `codegraph explore "<symbol names or question>"` prints the same output.

If there is no `.codegraph/` directory, skip CodeGraph entirely — indexing is the user's decision.
<!-- CODEGRAPH_END -->

## 运行时日志

Matrix 后端将运行日志按类别 **按天归档** 写入 `logs/{category}/{YYYY-MM-DD}.log`（根目录由 `config.yml` 的 `logging.dir` 决定；类别子目录名固定为 `system` / `api` / `llm` / `agent`）。

| 类别 | 目录 | 内容 |
|------|------|------|
| system | `logs/system/` | 启动、Job 队列调度、MCP 进程、settings、HTTP panic/5xx |
| api | `logs/api/` | 全部 HTTP API 的 nginx combined access 日志 |
| llm | `logs/llm/` | LLM HTTP 原始请求/响应（JSON Lines，`llm.request` / `llm.response`） |
| agent | `logs/agent/` | Agent TAOR 执行全链路、工具 I/O、Run 生命周期、LLM 调用错误（JSON Lines） |

**保留策略：** 日志目录已在 `.gitignore` 中排除（不提交仓库），但**本地磁盘上的日志必须保留**，用于事后排查。不要主动删除 `logs/` 下的文件，除非用户明确要求清理。过期文件由 `logging.retention_days` 自动清理。

**排查问题时：**

1. 按日期选择对应 `{category}/{date}.log`，或按关键字检索（`request_id`、`session_id`、`run_id`、`msg`）。
2. 关键字段：
   - `request_id` — 对应单次 HTTP 请求（`api.log`）
   - `session_id` / `run_id` — 对应 AI Run / 对话会话（`agent.log` / `llm.log`）
   - `msg=loop: 模型错误` / `run: LLM 调用失败` — 模型调用失败（`agent.log`）
   - `msg=loop: 工具结果` — 工具完整 input/output（`agent.log`）
3. 跨文件关联：同一 Run 用 `run_id` / `session_id` 串联 `agent.log` 与 `llm.log`；创建 Run 的 HTTP 请求用 `request_id` 关联 `api.log`。
4. 结合前端报错时间、Run ID、项目 ID 定位同一时段记录，再追溯根因。

配置项（见 `config/config.example.yml`）：

| 项 | 说明 |
|----|------|
| `logging.level` | system 日志级别：`debug` \| `info` \| `warn` \| `error` |
| `logging.retention_days` | 每类日志保留天数 |

## 日志与错误约定

**记一次（log once at origin）：**

- **LLM HTTP**：`llm.Client` 的 `RoundTripper` 在 Transport 层记一次原始 HTTP 请求/响应到 `llm.log`（`llm.request` / `llm.response`）；解析错误经 `return err` 向上传递，由 `query`（`loop: 模型错误`）与 `run`（`run: LLM 调用失败` / `run: 执行结束`）记入 `agent.log`；客户端错误摘要另记 `system.log`（`llm: 客户端错误`）。
- **HTTP**：access 进 `api.log`；panic 与 5xx 进 `system.log`（带 `request_id`）。
- **Agent**：TAOR 终态在 `queryLoop`（`loop: 结束`）；Run 生命周期、run-view、SSE 连接在 `execute` / `runtime` / `view`；工具 I/O 在 `query` / `tools`。

**分类：** 不跨类重复记录同一事件；上层传递 `error`，不在多处打相同明细。**Run 失败详情只在 `agent.log` 的 `run: 执行结束`；Job 队列仅记调度结果，不重复 Run 错误文本。**

**禁止：** 业务异常用 `fmt.Print*` 输出；`_ = err` 除非已在边界记录或确属可忽略路径（须注释说明）。

**关联字段：** `request_id`（HTTP）、`session_id` / `run_id`（Run/Agent）。

## 问题修复原则

修复 bug 或实现需求时，**先分析根因，再从根本上解决**，不要打补丁。

**根因分析：**

1. 复现问题，确认现象与触发条件（输入、状态、时序）。
2. 追溯调用链与数据流，定位**最早出错或设计缺陷**的位置，而非仅修复报错栈顶。
3. 结合 `logs/` 下各类别日志、测试、代码阅读，区分「表面症状」与「底层原因」。
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
