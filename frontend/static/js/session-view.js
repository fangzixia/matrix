/**
 * SessionView — Agent 流式消息（progress / stream_event / assistant / result）
 * P0–P2：统一时间轴、结构化工具卡、耗时、Worker 流式正文、增量渲染、Todo 清单
 */
(function (global) {
    const THROTTLE_MS = 100;
    const OUTPUT_PREVIEW_MAX = 500;
    const LOG_PREVIEW_MAX = 1200;
    const COLLAPSED_PREVIEW = 220;

    const TOOL_ICONS = {
        read: '📄',
        write: '✏️',
        edit: '✏️',
        list: '📁',
        grep: '🔍',
        glob: '🔍',
        bash: '⌨️',
        agent: '🤖',
        todo: '📋',
        mcp: '🔌',
        web: '🌐',
        default: '⚙️',
    };

    function pick(obj, ...keys) {
        if (!obj) return '';
        for (const k of keys) {
            const v = obj[k];
            if (v != null && v !== '') return v;
        }
        return '';
    }

    function normalizeProgressData(raw) {
        if (!raw) return {};
        return {
            type: pick(raw, 'type', 'Type'),
            status: pick(raw, 'status', 'Status'),
            turn: raw.turn ?? raw.Turn ?? 0,
            transition: pick(raw, 'transition', 'Transition'),
            summary: pick(raw, 'summary', 'Summary'),
            tool_name: pick(raw, 'tool_name', 'toolName', 'ToolName'),
            server_name: pick(raw, 'server_name', 'serverName', 'ServerName'),
            message: pick(raw, 'message', 'Message'),
            elapsed_time_ms: raw.elapsed_time_ms ?? raw.elapsedTimeMs ?? raw.ElapsedTimeMs ?? 0,
        };
    }

    function normalizeStreamMsg(msg) {
        if (!msg) return msg;
        const data = normalizeProgressData(msg.data || msg.Data);
        const assistant = msg.message || msg.Message;
        const event = msg.event || msg.Event;
        return {
            type: pick(msg, 'type', 'Type'),
            session_id: pick(msg, 'session_id', 'sessionId', 'SessionID'),
            scope: pick(msg, 'scope', 'Scope') || 'coordinator',
            agent_id: pick(msg, 'agent_id', 'agentId', 'AgentID'),
            parent_agent_id: pick(msg, 'parent_agent_id', 'parentAgentId', 'ParentAgentID'),
            tool_use_id: pick(msg, 'tool_use_id', 'toolUseId', 'ToolUseID'),
            data,
            event: event ? {
                type: pick(event, 'type', 'Type'),
                delta: (() => {
                    const d = event.delta || event.Delta;
                    if (!d) return {};
                    return {
                        type: pick(d, 'type', 'Type'),
                        text: pick(d, 'text', 'Text'),
                        thinking: pick(d, 'thinking', 'Thinking'),
                    };
                })(),
            } : undefined,
            message: assistant ? {
                content: assistant.content || assistant.Content || [],
                tool_calls: assistant.tool_calls || assistant.toolCalls || assistant.ToolCalls || [],
            } : undefined,
            is_error: msg.is_error ?? msg.isError ?? msg.IsError,
            stop_reason: pick(msg, 'stop_reason', 'stopReason', 'StopReason'),
            num_turns: msg.num_turns ?? msg.numTurns ?? msg.NumTurns,
            duration_ms: msg.duration_ms ?? msg.durationMs ?? msg.DurationMs ?? 0,
            output: pick(msg, 'output', 'Output'),
            error: pick(msg, 'error', 'Error'),
        };
    }

    function emptyState() {
        return {
            status: 'idle',
            sessionId: '',
            turn: 0,
            transition: '',
            thinkingText: '',
            assistantText: '',
            tools: {},
            feed: [],
            result: null,
            todos: [],
            durationMs: 0,
            activeToolKey: '',
        };
    }

    function emptyWorkerState(agentId) {
        const s = emptyState();
        s.agentId = agentId;
        s.status = 'streaming';
        return s;
    }

    function toolStorageKey(toolUseId) {
        return toolUseId || 'unknown';
    }

    function truncate(s, max = 400) {
        if (!s) return '';
        s = String(s);
        return s.length <= max ? s : s.slice(0, max) + '…';
    }

    function formatElapsed(ms) {
        const n = Number(ms) || 0;
        if (n <= 0) return '';
        if (n < 1000) return `${n}ms`;
        return `${(n / 1000).toFixed(n < 10000 ? 1 : 0)}s`;
    }

    function nowTimeStr() {
        return new Date().toLocaleTimeString('zh-CN', { hour12: false });
    }

    function escapeHtml(s) {
        const d = document.createElement('div');
        d.textContent = s || '';
        return d.innerHTML;
    }

    function toolIcon(name) {
        const n = (name || '').toLowerCase();
        if (n.startsWith('mcp_')) return TOOL_ICONS.mcp;
        if (n === 'agent' || n === 'send_message') return TOOL_ICONS.agent;
        if (n === 'todo_write') return TOOL_ICONS.todo;
        if (n.includes('read') || n === 'read_file') return TOOL_ICONS.read;
        if (n.includes('write') || n.includes('edit')) return TOOL_ICONS.write;
        if (n.includes('list') || n === 'list_dir') return TOOL_ICONS.list;
        if (n.includes('grep') || n.includes('glob')) return TOOL_ICONS.grep;
        if (n === 'bash' || n.includes('run_terminal')) return TOOL_ICONS.bash;
        if (n.includes('web') || n.includes('search')) return TOOL_ICONS.web;
        return TOOL_ICONS.default;
    }

    function tryParseJson(s) {
        if (!s || typeof s !== 'string') return null;
        try {
            return JSON.parse(s);
        } catch {
            return null;
        }
    }

    function toolSummaryLine(name, inputStr) {
        const args = tryParseJson(inputStr);
        if (!args) return truncate(inputStr, 120);
        const n = (name || '').toLowerCase();
        if (n === 'read_file' || n === 'write_file' || n === 'file_edit' || n === 'list_dir') {
            return args.path || args.file_path || args.directory || '';
        }
        if (n === 'grep' || n === 'glob') {
            return args.pattern || args.query || args.path || '';
        }
        if (n === 'bash') {
            return truncate(args.command || '', 100);
        }
        if (n === 'agent') {
            return args.description || truncate(args.prompt || '', 80);
        }
        if (n === 'todo_write') {
            const todos = args.todos;
            if (Array.isArray(todos)) return `更新 ${todos.length} 项任务`;
        }
        if (args.query) return truncate(String(args.query), 100);
        return truncate(inputStr, 120);
    }

    function parseAgentLaunch(inputStr, outputStr) {
        const args = tryParseJson(inputStr);
        let agentId = '';
        const m = (outputStr || '').match(/<agent_id>([^<]+)<\/agent_id>/);
        if (m) agentId = m[1].trim();
        return {
            description: args?.description || '',
            agentId,
        };
    }

    function parseTodosFromOutput(text) {
        if (!text || !text.includes('TODO')) return null;
        const items = [];
        const re = /[○◐●✕?]\s*\[([^\]]+)\]\s*(\w+):\s*(.+)/g;
        let m;
        while ((m = re.exec(text)) !== null) {
            items.push({ id: m[1], status: m[2], content: m[3].trim() });
        }
        return items.length ? items : null;
    }

    function createToolRecord(toolUseId, data) {
        const name = data?.tool_name || 'tool';
        return {
            id: toolUseId,
            name,
            serverName: data?.server_name || '',
            status: 'running',
            summary: '',
            input: '',
            output: '',
            errorMsg: '',
            elapsedMs: 0,
            startedAt: Date.now(),
            agentLaunch: null,
            expandInput: false,
            expandOutput: false,
        };
    }

    function migrateToolFields(t) {
        if (!t) return;
        if (t.log && !t.input && !t.output) {
            t.input = t.log;
            delete t.log;
        }
        delete t.inputPreview;
        delete t.outputPreview;
        if (!t.summary && t.input) {
            t.summary = toolSummaryLine(t.name, t.input);
        }
    }

    function statusLabel(status) {
        if (status === 'running') return '执行中';
        if (status === 'failed') return '失败';
        return '完成';
    }

    function applyToolProgress(t, data) {
        if (!t) return;
        if (data.tool_name && (t.name === 'tool' || !t.name)) t.name = data.tool_name;
        if (data.server_name) t.serverName = data.server_name;
        if (data.elapsed_time_ms) t.elapsedMs = data.elapsed_time_ms;

        const isMcp = data.type === 'mcp_progress';

        if (data.status === 'started') {
            t.status = 'running';
            if (data.message) {
                t.input = data.message;
                t.summary = toolSummaryLine(t.name, data.message);
                if (t.name === 'agent') {
                    t.agentLaunch = parseAgentLaunch(data.message, '');
                }
            }
            return;
        }

        if (data.status === 'completed' || data.status === 'failed') {
            t.status = data.status === 'failed' ? 'failed' : 'success';
            if (data.message) {
                if (data.status === 'failed') {
                    t.errorMsg = truncate(data.message, OUTPUT_PREVIEW_MAX);
                    t.output = data.message;
                } else {
                    t.output = data.message;
                }
            }
            if (t.name === 'agent' && data.message) {
                const launch = parseAgentLaunch(t.input, data.message);
                t.agentLaunch = launch;
                if (launch.description) t.summary = launch.description;
            }
            if (data.elapsed_time_ms) t.elapsedMs = data.elapsed_time_ms;
        }

        if (isMcp && data.message) {
            if (data.status === 'started') t.input = data.message;
            else t.output = (t.output ? t.output + '\n\n' : '') + data.message;
        }
    }

    function buildFeedFromLegacy(snap) {
        const feed = [];
        (snap.timeline || []).forEach((entry) => {
            feed.push({
                kind: entry.kind === 'done' || entry.kind === 'error' ? entry.kind : 'turn',
                time: entry.time,
                summary: entry.summary,
            });
        });
        Object.keys(snap.tools || {}).forEach((toolKey) => {
            feed.push({ kind: 'tool', toolKey });
        });
        return feed;
    }

    function computeSessionStats(state, workers) {
        let toolCalls = Object.keys(state.tools || {}).length;
        let workerAgents = 0;
        Object.values(workers || {}).forEach((ui) => {
            workerAgents += 1;
            toolCalls += Object.keys(ui.state.tools || {}).length;
        });
        const turns = state.turn || 0;
        return { turns, toolCalls, workerAgents };
    }

    function formatStepCountLabel(stats) {
        const parts = [];
        if (stats.turns > 0) parts.push(`${stats.turns} 轮`);
        if (stats.toolCalls > 0) parts.push(`${stats.toolCalls} 次工具`);
        if (stats.workerAgents > 0) parts.push(`${stats.workerAgents} 个 Worker`);
        return parts.length ? parts.join(' · ') : '0 步';
    }

    /** 增量渲染 Feed：工具卡复用 DOM，仅更新变化字段 */
    function createFeedRenderer(containerEl, resolveTool, options = {}) {
        const cardMap = new Map();
        const rowMap = new Map();

        function makeExpandablePre(text, expanded, onToggle) {
            const wrap = document.createElement('div');
            wrap.className = 'tool-io-block';
            const pre = document.createElement('pre');
            pre.className = 'tool-card-body';
            const full = text || '（无内容）';
            const needsClamp = full.length > COLLAPSED_PREVIEW;
            pre.textContent = expanded || !needsClamp ? full : truncate(full, COLLAPSED_PREVIEW);
            wrap.appendChild(pre);
            if (needsClamp) {
                const btn = document.createElement('button');
                btn.type = 'button';
                btn.className = 'tool-expand-btn';
                btn.textContent = expanded ? '收起' : '展开全部';
                btn.addEventListener('click', (e) => {
                    e.preventDefault();
                    onToggle();
                });
                wrap.appendChild(btn);
            }
            return wrap;
        }

        function updateToolCardEl(card, t) {
            migrateToolFields(t);
            const isActive = options.getActiveToolKey?.() === toolStorageKey(t.id);
            card.className = `tool-card tool-card-${t.status} tool-kind-${(t.name || '').replace(/[^a-z0-9_]/gi, '_')} ${isActive ? 'tool-card-active' : ''}`;

            const iconEl = card.querySelector('.tool-card-icon');
            if (iconEl) iconEl.textContent = toolIcon(t.name);

            const titleEl = card.querySelector('.tool-card-title');
            if (titleEl) titleEl.textContent = t.name + (t.serverName ? ` (${t.serverName})` : '');

            const hintEl = card.querySelector('.tool-card-hint');
            if (hintEl) hintEl.textContent = t.summary || '';

            const metaEl = card.querySelector('.tool-card-meta');
            if (metaEl) {
                const elapsed = formatElapsed(t.elapsedMs);
                const spin = t.status === 'running' ? '<span class="tool-spinner" aria-hidden="true"></span> ' : '';
                metaEl.innerHTML = `${spin}${escapeHtml(statusLabel(t.status))}${elapsed ? ` · ${escapeHtml(elapsed)}` : ''}`;
            }

            let sub = card.querySelector('.tool-subagent-block');
            if (t.agentLaunch?.agentId || t.name === 'agent') {
                if (!sub) {
                    sub = document.createElement('div');
                    sub.className = 'tool-subagent-block';
                    card.querySelector('.tool-card-body-wrap')?.prepend(sub);
                }
                const aid = t.agentLaunch?.agentId || '';
                sub.innerHTML = `
                    <div class="tool-subagent-title">子任务</div>
                    <div class="tool-subagent-desc">${escapeHtml(t.agentLaunch?.description || t.summary || '')}</div>
                    ${aid ? `<code class="tool-subagent-id">${escapeHtml(aid)}</code>` : ''}
                    ${aid ? `<button type="button" class="btn btn-secondary btn-sm tool-jump-worker" data-agent-id="${escapeHtml(aid)}">查看 Worker 步骤</button>` : ''}
                `;
                sub.querySelector('.tool-jump-worker')?.addEventListener('click', (e) => {
                    e.preventDefault();
                    options.onJumpToWorker?.(aid);
                });
            } else if (sub) {
                sub.remove();
            }

            const errEl = card.querySelector('.tool-card-error');
            if (t.errorMsg && t.status === 'failed') {
                if (!errEl) {
                    const err = document.createElement('div');
                    err.className = 'tool-card-error';
                    card.querySelector('.tool-card-body-wrap')?.prepend(err);
                }
                card.querySelector('.tool-card-error').textContent = t.errorMsg;
            } else if (errEl) {
                errEl.remove();
            }

            const bodyWrap = card.querySelector('.tool-card-body-wrap');
            if (!bodyWrap) return;

            bodyWrap.querySelectorAll('[data-io-section]').forEach((el) => el.remove());

            if (t.input) {
                const sec = document.createElement('div');
                sec.dataset.ioSection = 'input';
                const lab = document.createElement('div');
                lab.className = 'tool-io-label';
                lab.textContent = '输入';
                sec.appendChild(lab);
                sec.appendChild(makeExpandablePre(t.input, t.expandInput, () => {
                    t.expandInput = !t.expandInput;
                    updateToolCardEl(card, t);
                }));
                bodyWrap.appendChild(sec);
            }

            if (t.output && t.status !== 'running') {
                const sec = document.createElement('div');
                sec.dataset.ioSection = 'output';
                const lab = document.createElement('div');
                lab.className = 'tool-io-label';
                lab.textContent = '输出';
                sec.appendChild(lab);
                sec.appendChild(makeExpandablePre(t.output, t.expandOutput, () => {
                    t.expandOutput = !t.expandOutput;
                    updateToolCardEl(card, t);
                }));
                bodyWrap.appendChild(sec);
            }
        }

        function createToolCardEl(t) {
            migrateToolFields(t);
            const card = document.createElement('details');
            card.className = `tool-card tool-card-${t.status}`;
            card.dataset.toolKey = toolStorageKey(t.id);
            card.open = false;

            const summary = document.createElement('summary');
            summary.className = 'tool-card-summary';
            summary.innerHTML = `
                <span class="tool-card-icon" aria-hidden="true">${toolIcon(t.name)}</span>
                <span class="tool-card-title-wrap">
                    <span class="tool-card-title"></span>
                    <span class="tool-card-hint"></span>
                </span>
                <span class="tool-card-meta"></span>
            `;

            const bodyWrap = document.createElement('div');
            bodyWrap.className = 'tool-card-body-wrap';

            card.appendChild(summary);
            card.appendChild(bodyWrap);
            updateToolCardEl(card, t);
            return card;
        }

        function createTimelineRow(entry) {
            const row = document.createElement('div');
            const kind = entry.kind || 'turn';
            row.className = `timeline-entry timeline-${kind}`;
            row.dataset.feedKey = `${kind}:${entry.time || ''}:${entry.summary || ''}`;
            row.innerHTML = `
                <span class="timeline-time">${escapeHtml(entry.time || '')}</span>
                <span class="timeline-text">${escapeHtml(entry.summary || '')}</span>
            `;
            return row;
        }

        function sync(feed) {
            const stickBottom = containerEl.scrollTop + containerEl.clientHeight >= containerEl.scrollHeight - 48;
            const seenTools = new Set();
            const seenRows = new Set();
            const frag = document.createDocumentFragment();

            (feed || []).forEach((entry) => {
                if (entry.kind === 'tool') {
                    seenTools.add(entry.toolKey);
                    const t = resolveTool(entry.toolKey);
                    if (!t) return;
                    let card = cardMap.get(entry.toolKey);
                    if (!card) {
                        card = createToolCardEl(t);
                        cardMap.set(entry.toolKey, card);
                    } else {
                        updateToolCardEl(card, t);
                    }
                    frag.appendChild(card);
                    return;
                }

                const rowKey = `${entry.kind}:${entry.time}:${entry.summary}`;
                seenRows.add(rowKey);
                let row = rowMap.get(rowKey);
                if (!row) {
                    row = createTimelineRow(entry);
                    rowMap.set(rowKey, row);
                }
                frag.appendChild(row);
            });

            cardMap.forEach((card, key) => {
                if (!seenTools.has(key)) cardMap.delete(key);
            });
            rowMap.forEach((row, key) => {
                if (!seenRows.has(key)) rowMap.delete(key);
            });

            containerEl.replaceChildren(frag);

            if (stickBottom) {
                containerEl.scrollTop = containerEl.scrollHeight;
            }
        }

        return { sync, updateToolCardEl, createToolCardEl };
    }

    function createSessionView(rootEl, options = {}) {
        const compact = !!options.compact;
        const subagentPanel = options.subagentPanel || null;
        const onJumpToWorker = options.onJumpToWorker || null;
        const state = emptyState();
        const workers = {};
        let markdownThrottle = null;

        if (!rootEl) {
            return {
                apply() {},
                reset() {},
                getSnapshot: () => ({ ...state }),
                finalizeRunningTools() {},
                scrollToWorker() {},
                getStepCountLabel: () => '0 步',
            };
        }

        rootEl.classList.add('session-view');
        if (compact) rootEl.classList.add('session-view-compact');

        const phaseEl = document.createElement('div');
        phaseEl.className = 'session-phase';
        const currentStepEl = document.createElement('div');
        currentStepEl.className = 'session-current-step';
        const todosEl = document.createElement('div');
        todosEl.className = 'session-todos';
        todosEl.hidden = true;
        const streamEl = document.createElement('div');
        streamEl.className = 'session-stream';
        const thinkingWrap = document.createElement('details');
        thinkingWrap.className = 'session-thinking';
        thinkingWrap.innerHTML = '<summary>思考过程</summary><pre class="session-thinking-body"></pre>';
        const assistantEl = document.createElement('div');
        assistantEl.className = 'session-assistant markdown-content';
        const feedEl = document.createElement('div');
        feedEl.className = 'session-feed';
        const workersEl = document.createElement('div');
        workersEl.className = 'session-workers';
        workersEl.hidden = true;

        rootEl.appendChild(phaseEl);
        rootEl.appendChild(currentStepEl);
        rootEl.appendChild(todosEl);
        rootEl.appendChild(workersEl);
        rootEl.appendChild(streamEl);
        rootEl.appendChild(feedEl);

        const thinkingBody = thinkingWrap.querySelector('.session-thinking-body');

        const feedRenderer = createFeedRenderer(
            feedEl,
            (key) => state.tools[key] || findToolByKey(key),
            {
                getActiveToolKey: () => state.activeToolKey,
                onJumpToWorker: (agentId) => scrollToWorker(agentId),
            }
        );

        function findToolByKey(key) {
            if (state.tools[key]) return state.tools[key];
            for (const ui of Object.values(workers)) {
                if (ui.state.tools[key]) return ui.state.tools[key];
            }
            return null;
        }

        function findRunningToolKey(toolsMap) {
            let last = '';
            Object.entries(toolsMap || {}).forEach(([key, t]) => {
                if (t.status === 'running') last = key;
            });
            return last;
        }

        function updateCurrentStep() {
            let key = findRunningToolKey(state.tools);
            if (!key) {
                for (const ui of Object.values(workers)) {
                    const wk = findRunningToolKey(ui.state.tools);
                    if (wk) {
                        key = wk;
                        break;
                    }
                }
            }
            state.activeToolKey = key;
            const t = key ? (state.tools[key] || findToolByKey(key)) : null;
            if (t && t.status === 'running') {
                currentStepEl.hidden = false;
                currentStepEl.innerHTML = `<span class="tool-spinner"></span> 当前：<strong>${escapeHtml(t.name)}</strong>${t.summary ? ` — ${escapeHtml(t.summary)}` : ''}`;
            } else if (state.status === 'streaming') {
                currentStepEl.hidden = false;
                currentStepEl.textContent = '等待模型响应…';
            } else {
                currentStepEl.hidden = true;
            }
        }

        function renderTodos() {
            if (!state.todos || state.todos.length === 0) {
                todosEl.hidden = true;
                todosEl.innerHTML = '';
                return;
            }
            todosEl.hidden = false;
            const done = state.todos.filter((x) => x.status === 'completed').length;
            todosEl.innerHTML = `
                <div class="session-todos-header">任务清单 <span class="session-todos-count">${done}/${state.todos.length}</span></div>
                <ul class="session-todos-list">
                    ${state.todos.map((item) => `
                        <li class="todo-item todo-${escapeHtml(item.status)}">
                            <span class="todo-status-icon">${item.status === 'completed' ? '●' : item.status === 'in_progress' ? '◐' : '○'}</span>
                            <span class="todo-content">${escapeHtml(item.content)}</span>
                        </li>
                    `).join('')}
                </ul>
            `;
        }

        function scrollToWorker(agentId) {
            const ui = workers[agentId];
            if (!ui) return;
            ui.el.open = true;
            ui.el.scrollIntoView({ behavior: 'smooth', block: 'nearest' });
            if (typeof onJumpToWorker === 'function') onJumpToWorker(agentId);
        }

        function getWorkerUI(agentId, description) {
            if (!agentId) return null;
            if (workers[agentId]) return workers[agentId];
            const wrap = document.createElement('details');
            wrap.className = 'worker-stream-panel';
            wrap.open = true;
            wrap.dataset.agentId = agentId;
            wrap.innerHTML = `
                <summary class="worker-stream-summary">
                    <span class="worker-stream-label">Worker</span>
                    <code class="worker-stream-id"></code>
                </summary>
                <div class="worker-stream-body">
                    <div class="worker-phase"></div>
                    <div class="worker-current-step"></div>
                    <details class="worker-thinking session-thinking">
                        <summary>思考过程</summary>
                        <pre class="worker-thinking-body"></pre>
                    </details>
                    <div class="worker-assistant markdown-content"></div>
                    <div class="worker-feed session-feed"></div>
                </div>
            `;
            wrap.querySelector('.worker-stream-id').textContent = agentId;
            if (description) {
                const desc = document.createElement('div');
                desc.className = 'worker-desc';
                desc.textContent = description;
                wrap.querySelector('.worker-stream-body').prepend(desc);
            }
            workersEl.appendChild(wrap);
            workersEl.hidden = false;

            const ui = {
                el: wrap,
                state: emptyWorkerState(agentId),
                phaseEl: wrap.querySelector('.worker-phase'),
                currentStepEl: wrap.querySelector('.worker-current-step'),
                thinkingBody: wrap.querySelector('.worker-thinking-body'),
                thinkingWrap: wrap.querySelector('.worker-thinking'),
                assistantEl: wrap.querySelector('.worker-assistant'),
                feedEl: wrap.querySelector('.worker-feed'),
                feedRenderer: null,
            };
            ui.feedRenderer = createFeedRenderer(
                ui.feedEl,
                (key) => ui.state.tools[key],
                {
                    getActiveToolKey: () => findRunningToolKey(ui.state.tools),
                    onJumpToWorker: () => scrollToWorker(agentId),
                }
            );
            workers[agentId] = ui;
            return ui;
        }

        function renderWorkerPhase(ui) {
            const s = ui.state;
            const phaseLabel = s.status === 'done' ? '已完成' : s.status === 'error' ? '失败' : '执行中';
            const turnPart = s.turn > 0 ? `第 ${s.turn} 轮` : '';
            const trans = s.transition ? ` · ${s.transition}` : '';
            ui.phaseEl.textContent = `${phaseLabel}${turnPart ? ' · ' + turnPart : ''}${trans}`;
        }

        function renderWorkerCurrentStep(ui) {
            const key = findRunningToolKey(ui.state.tools);
            const t = key ? ui.state.tools[key] : null;
            if (t && t.status === 'running') {
                ui.currentStepEl.hidden = false;
                ui.currentStepEl.innerHTML = `<span class="tool-spinner"></span> 当前工具：<strong>${escapeHtml(t.name)}</strong>${t.summary ? ` — ${escapeHtml(t.summary)}` : ''}`;
            } else {
                ui.currentStepEl.hidden = true;
            }
        }

        function renderWorkerMarkdown(ui) {
            const renderMd = global.formatChatMarkdown || global.formatMarkdown;
            if (typeof renderMd === 'function') {
                ui.assistantEl.innerHTML = renderMd(ui.state.assistantText || '');
            } else {
                ui.assistantEl.textContent = ui.state.assistantText || '';
            }
        }

        function renderWorkerFeed(ui) {
            ui.feedRenderer.sync(ui.state.feed || []);
            renderWorkerCurrentStep(ui);
            scrollToLatest();
        }

        function ensureFeedToolIn(feed, key) {
            if (!feed.some((e) => e.kind === 'tool' && e.toolKey === key)) {
                feed.push({ kind: 'tool', toolKey: key });
            }
        }

        function getOrCreateTool(targetTools, feed, toolUseId, data) {
            const key = toolStorageKey(toolUseId);
            let t = targetTools[key];
            if (!t) {
                t = createToolRecord(toolUseId, data);
                targetTools[key] = t;
                ensureFeedToolIn(feed, key);
            }
            return t;
        }

        function applyToolProgressToMaps(targetTools, feed, toolUseId, data) {
            const isToolProgress = data.type === 'tool_progress' || data.type === 'mcp_progress'
                || (data.status && data.type !== 'turn_progress');
            if (!isToolProgress) return null;

            const t = getOrCreateTool(targetTools, feed, toolUseId, data);
            applyToolProgress(t, data);

            if (t.name === 'todo_write' && t.output) {
                const todos = parseTodosFromOutput(t.output);
                if (todos) state.todos = todos;
            }
            if (t.agentLaunch?.agentId) {
                getWorkerUI(t.agentLaunch.agentId, t.agentLaunch.description || t.summary);
            }
            return t;
        }

        const toolProgressHandlers = {
            toolProgress(targetState, targetTools, feed, toolUseId, data) {
                applyToolProgressToMaps(targetTools, feed, toolUseId, data);
            },
        };

        function applyToState(targetState, targetTools, feed, msg, handlers) {
            const data = msg.data || {};
            if (data.type === 'turn_progress') {
                targetState.status = 'streaming';
                targetState.turn = data.turn || targetState.turn;
                targetState.transition = data.transition || '';
                if (data.summary) {
                    feed.push({
                        kind: 'turn',
                        time: nowTimeStr(),
                        summary: `${msg.agent_id ? `[${msg.agent_id}] ` : ''}${data.summary}${data.transition ? ` (${data.transition})` : ''}`,
                    });
                }
                return;
            }
            const toolUseId = msg.tool_use_id;
            if (!toolUseId) return;
            handlers.toolProgress(targetState, targetTools, feed, toolUseId, data, msg);
        }

        function applyAssistantToolsToState(targetTools, feed, msg) {
            (msg.message?.tool_calls || []).forEach((tc) => {
                const id = pick(tc, 'id', 'Id', 'ID');
                const name = pick(tc, 'name', 'Name') || 'tool';
                const input = pick(tc, 'input', 'Input', 'arguments', 'Arguments');
                const key = toolStorageKey(id);
                if (!targetTools[key]) {
                    targetTools[key] = createToolRecord(id, { tool_name: name });
                }
                const t = targetTools[key];
                t.name = name;
                if (input) {
                    t.input = input;
                    t.summary = toolSummaryLine(name, input);
                    if (name === 'agent') t.agentLaunch = parseAgentLaunch(input, '');
                }
                ensureFeedToolIn(feed, key);
            });
        }

        function applyWorkerStreamEvent(ui, msg) {
            ui.state.status = 'streaming';
            const delta = msg.event?.delta || {};
            if (delta.type === 'thinking_delta' && delta.thinking) {
                ui.state.thinkingText += delta.thinking;
                ui.thinkingBody.textContent = ui.state.thinkingText;
                ui.thinkingWrap.open = true;
            }
            if (delta.type === 'text_delta' && delta.text) {
                ui.state.assistantText += delta.text;
                renderWorkerMarkdown(ui);
            }
        }

        function shouldStickBottom(el) {
            if (!el) return true;
            return el.scrollTop + el.clientHeight >= el.scrollHeight - 48;
        }

        function scrollToLatest(force) {
            if (!rootEl) return;
            if (force || shouldStickBottom(rootEl)) {
                requestAnimationFrame(() => {
                    rootEl.scrollTop = rootEl.scrollHeight;
                });
            }
        }

        function renderPhase() {
            const phaseLabel = state.status === 'done' ? '已完成' : state.status === 'error' ? '失败' : '执行中';
            const turnPart = state.turn > 0 ? `第 ${state.turn} 轮` : '';
            const transPart = state.transition ? ` · ${state.transition}` : '';
            const dur = state.durationMs ? ` · ${formatElapsed(state.durationMs)}` : '';
            phaseEl.textContent = `${phaseLabel}${turnPart ? ' · ' + turnPart : ''}${transPart}${dur}`;
            updateCurrentStep();
            renderTodos();
            scrollToLatest();
        }

        function scheduleMarkdown() {
            const renderMd = global.formatChatMarkdown || global.formatMarkdown;
            if (typeof renderMd !== 'function') {
                assistantEl.textContent = state.assistantText;
                return;
            }
            if (markdownThrottle) return;
            markdownThrottle = setTimeout(() => {
                markdownThrottle = null;
                assistantEl.innerHTML = renderMd(state.assistantText || '');
                scrollToLatest();
            }, THROTTLE_MS);
        }

        function renderFeed() {
            feedRenderer.sync(state.feed || []);
            updateCurrentStep();
            scrollToLatest();
        }

        function pushFeed(entry) {
            state.feed.push(entry);
            renderFeed();
        }

        function apply(rawMsg) {
            const msg = normalizeStreamMsg(rawMsg);
            if (!msg || !msg.type) return;

            if (msg.session_id) state.sessionId = msg.session_id;

            if (msg.scope === 'worker' && msg.agent_id) {
                const ui = getWorkerUI(msg.agent_id);
                if (msg.type === 'progress') {
                    applyToState(ui.state, ui.state.tools, ui.state.feed, msg, toolProgressHandlers);
                    renderWorkerPhase(ui);
                    renderWorkerFeed(ui);
                    renderPhase();
                    return;
                }
                if (msg.type === 'result') {
                    ui.state.status = msg.is_error ? 'error' : 'done';
                    if (msg.duration_ms) ui.state.durationMs = msg.duration_ms;
                    const toolFinal = msg.is_error ? 'failed' : 'success';
                    Object.values(ui.state.tools).forEach((t) => {
                        if (t.status === 'running') t.status = toolFinal;
                    });
                    if (msg.output) ui.state.assistantText = msg.output;
                    renderWorkerPhase(ui);
                    renderWorkerMarkdown(ui);
                    renderWorkerFeed(ui);
                    renderPhase();
                    return;
                }
                if (msg.type === 'assistant') {
                    ui.state.status = 'streaming';
                    const m = msg.message || {};
                    (m.content || []).forEach((block) => {
                        if (block.type === 'thinking' && block.thinking) {
                            ui.state.thinkingText = block.thinking;
                            ui.thinkingBody.textContent = ui.state.thinkingText;
                        }
                        if (block.type === 'text' && block.text) {
                            ui.state.assistantText = block.text;
                        }
                    });
                    applyAssistantToolsToState(ui.state.tools, ui.state.feed, msg);
                    renderWorkerMarkdown(ui);
                    renderWorkerPhase(ui);
                    renderWorkerFeed(ui);
                    renderPhase();
                    return;
                }
                if (msg.type === 'stream_event') {
                    applyWorkerStreamEvent(ui, msg);
                    renderWorkerPhase(ui);
                    return;
                }
            }

            switch (msg.type) {
                case 'progress':
                    applyProgress(msg);
                    break;
                case 'stream_event':
                    applyStreamEvent(msg);
                    break;
                case 'assistant':
                    applyAssistant(msg);
                    break;
                case 'result':
                    applyResult(msg);
                    break;
                default:
                    break;
            }
            renderPhase();
        }

        function applyProgress(msg) {
            const data = msg.data || {};
            if (data.type === 'turn_progress') {
                state.status = 'streaming';
                state.turn = data.turn || state.turn;
                state.transition = data.transition || '';
                if (data.summary) {
                    pushFeed({
                        kind: 'turn',
                        time: nowTimeStr(),
                        summary: `${data.summary}${data.transition ? ` (${data.transition})` : ''}`,
                    });
                }
                return;
            }
            const toolUseId = msg.tool_use_id;
            if (!toolUseId) return;
            applyToolProgressToMaps(state.tools, state.feed, toolUseId, data);
            renderFeed();
        }

        function applyStreamEvent(msg) {
            state.status = 'streaming';
            const delta = msg.event?.delta || {};
            if (delta.type === 'thinking_delta' && delta.thinking) {
                state.thinkingText += delta.thinking;
                thinkingBody.textContent = state.thinkingText;
                thinkingWrap.open = true;
            }
            if (delta.type === 'text_delta' && delta.text) {
                state.assistantText += delta.text;
                scheduleMarkdown();
                scrollToLatest();
            }
        }

        function applyAssistant(msg) {
            const m = msg.message || {};
            (m.content || []).forEach((block) => {
                if (block.type === 'thinking' && block.thinking) {
                    state.thinkingText = block.thinking;
                    thinkingBody.textContent = state.thinkingText;
                }
                if (block.type === 'text' && block.text) {
                    state.assistantText = block.text;
                }
            });
            applyAssistantToolsToState(state.tools, state.feed, msg);
            scheduleMarkdown();
            renderFeed();
        }

        function finalizeRunningTools(outcome) {
            const toolFinal = outcome === 'error' ? 'failed' : 'success';
            Object.values(state.tools).forEach((t) => {
                if (t.status === 'running') t.status = toolFinal;
            });
            Object.values(workers).forEach((ui) => {
                Object.values(ui.state.tools).forEach((t) => {
                    if (t.status === 'running') t.status = toolFinal;
                });
                renderWorkerFeed(ui);
            });
            state.activeToolKey = '';
            renderFeed();
        }

        function applyResult(msg) {
            state.status = msg.is_error ? 'error' : 'done';
            if (msg.duration_ms) state.durationMs = msg.duration_ms;
            finalizeRunningTools(state.status);
            state.result = {
                subtype: msg.subtype,
                stopReason: msg.stop_reason,
                numTurns: msg.num_turns,
                durationMs: msg.duration_ms,
                output: msg.output,
                error: msg.error,
            };
            if (msg.output) {
                state.assistantText = msg.output;
                scheduleMarkdown();
            }
            const stats = computeSessionStats(state, workers);
            const dur = formatElapsed(msg.duration_ms);
            const label = msg.is_error
                ? `结束: ${msg.error || msg.stop_reason || 'error'}`
                : `完成 · ${formatStepCountLabel(stats)} · ${msg.stop_reason || 'ok'}${dur ? ` · 耗时 ${dur}` : ''}`;
            pushFeed({
                kind: msg.is_error ? 'error' : 'done',
                time: nowTimeStr(),
                summary: label,
            });
        }

        function serializeWorkers() {
            const out = {};
            Object.entries(workers).forEach(([id, ui]) => {
                out[id] = {
                    agentId: id,
                    state: JSON.parse(JSON.stringify(ui.state)),
                };
            });
            return out;
        }

        function restoreWorkers(saved) {
            workersEl.innerHTML = '';
            workersEl.hidden = true;
            Object.keys(workers).forEach((k) => delete workers[k]);
            if (!saved) return;
            Object.entries(saved).forEach(([id, pack]) => {
                const ui = getWorkerUI(id);
                Object.assign(ui.state, pack.state || {});
                Object.values(ui.state.tools || {}).forEach(migrateToolFields);
                if (!ui.state.feed?.length) {
                    ui.state.feed = buildFeedFromLegacy(ui.state);
                }
                renderWorkerPhase(ui);
                ui.thinkingBody.textContent = ui.state.thinkingText || '';
                ui.thinkingWrap.open = !!ui.state.thinkingText;
                renderWorkerMarkdown(ui);
                renderWorkerFeed(ui);
            });
        }

        function reset() {
            Object.assign(state, emptyState());
            phaseEl.textContent = '';
            currentStepEl.hidden = true;
            todosEl.hidden = true;
            thinkingBody.textContent = '';
            assistantEl.innerHTML = '';
            feedEl.innerHTML = '';
            workersEl.innerHTML = '';
            workersEl.hidden = true;
            Object.keys(workers).forEach((k) => delete workers[k]);
            if (subagentPanel) subagentPanel.clear();
            state.status = 'streaming';
            renderPhase();
            scrollToLatest(true);
        }

        function getSnapshot() {
            return JSON.parse(JSON.stringify({
                status: state.status,
                sessionId: state.sessionId,
                turn: state.turn,
                transition: state.transition,
                thinkingText: state.thinkingText,
                assistantText: state.assistantText,
                tools: state.tools,
                feed: state.feed,
                result: state.result,
                todos: state.todos,
                durationMs: state.durationMs,
                workers: serializeWorkers(),
                stats: computeSessionStats(state, workers),
            }));
        }

        function loadSnapshot(snap) {
            if (!snap) {
                reset();
                return;
            }
            Object.assign(state, emptyState(), snap);
            Object.values(state.tools || {}).forEach(migrateToolFields);
            if (!state.feed?.length) {
                state.feed = buildFeedFromLegacy(snap);
            }
            if (state.status === 'done' || state.status === 'error') {
                finalizeRunningTools(state.status);
            }
            thinkingBody.textContent = state.thinkingText || '';
            thinkingWrap.open = !!state.thinkingText;
            scheduleMarkdown();
            restoreWorkers(snap.workers);
            renderFeed();
            renderPhase();
            scrollToLatest(true);
        }

        function getStepCountLabel() {
            return formatStepCountLabel(computeSessionStats(state, workers));
        }

        global.SessionViewScrollToWorker = scrollToWorker;

        return {
            apply,
            reset,
            getSnapshot,
            loadSnapshot,
            getState: () => state,
            finalizeRunningTools,
            scrollToWorker,
            getStepCountLabel,
        };
    }

    global.SessionView = {
        create: createSessionView,
        emptyState,
        formatStepCountLabel,
        computeSessionStats,
    };
})(window);
