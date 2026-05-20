/**
 * SessionView — Agent 流式消息（progress / stream_event / assistant / result）
 */
(function (global) {
    const THROTTLE_MS = 100;

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
        };
    }

    /** Wails 事件可能使用 camelCase / PascalCase，统一为 snake_case 形态 */
    function normalizeStreamMsg(msg) {
        if (!msg) return msg;
        const data = normalizeProgressData(msg.data || msg.Data);
        const assistant = msg.message || msg.Message;
        const event = msg.event || msg.Event;
        return {
            type: pick(msg, 'type', 'Type'),
            session_id: pick(msg, 'session_id', 'sessionId', 'SessionID'),
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
        };
    }

    function toolStorageKey(toolUseId) {
        return toolUseId || 'unknown';
    }

    function truncate(s, max = 400) {
        if (!s) return '';
        s = String(s);
        return s.length <= max ? s : s.slice(0, max) + '…';
    }

    const OUTPUT_PREVIEW_MAX = 500;
    const LOG_PREVIEW_MAX = 1200;

    function appendToolLog(tool, text, maxLen) {
        if (!text) return;
        const chunk = truncate(text, maxLen || 400);
        if (tool.log) {
            if (tool.log === chunk || tool.log.endsWith('\n\n' + chunk)) return;
            tool.log = tool.log + '\n\n' + chunk;
        } else {
            tool.log = chunk;
        }
    }

    function migrateToolFields(t) {
        if (!t.log && (t.inputPreview || t.outputPreview)) {
            const parts = [];
            if (t.inputPreview) parts.push(t.inputPreview);
            if (t.outputPreview) parts.push(t.outputPreview);
            t.log = parts.join('\n\n');
        }
        delete t.inputPreview;
        delete t.outputPreview;
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

    function createSessionView(rootEl, options = {}) {
        const compact = !!options.compact;
        const state = emptyState();
        let markdownThrottle = null;

        if (!rootEl) {
            return {
                apply() {},
                reset() {},
                getSnapshot: () => ({ ...state, tools: { ...state.tools } }),
            };
        }

        rootEl.classList.add('session-view');
        if (compact) rootEl.classList.add('session-view-compact');

        const phaseEl = document.createElement('div');
        phaseEl.className = 'session-phase';
        const streamEl = document.createElement('div');
        streamEl.className = 'session-stream';
        const thinkingWrap = document.createElement('details');
        thinkingWrap.className = 'session-thinking';
        thinkingWrap.innerHTML = '<summary>思考过程</summary><pre class="session-thinking-body"></pre>';
        const assistantEl = document.createElement('div');
        assistantEl.className = 'session-assistant markdown-content';
        const feedEl = document.createElement('div');
        feedEl.className = 'session-feed';

        streamEl.appendChild(thinkingWrap);
        streamEl.appendChild(assistantEl);
        rootEl.appendChild(phaseEl);
        rootEl.appendChild(streamEl);
        rootEl.appendChild(feedEl);

        const thinkingBody = thinkingWrap.querySelector('.session-thinking-body');

        function renderPhase() {
            const phaseLabel = state.status === 'done' ? '已完成' : state.status === 'error' ? '失败' : '执行中';
            const turnPart = state.turn > 0 ? `第 ${state.turn} 轮` : '';
            const transPart = state.transition ? ` · ${state.transition}` : '';
            phaseEl.textContent = `${phaseLabel}${turnPart ? ' · ' + turnPart : ''}${transPart}`;
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
            }, THROTTLE_MS);
        }

        function escapeHtml(s) {
            const d = document.createElement('div');
            d.textContent = s;
            return d.innerHTML;
        }

        function findTool(toolUseId) {
            if (!toolUseId) return null;
            const key = toolStorageKey(toolUseId);
            if (state.tools[key]) return state.tools[key];
            return Object.values(state.tools).find((t) => t.id === toolUseId) || null;
        }

        function createToolCardEl(t) {
            migrateToolFields(t);
            const details = document.createElement('details');
            details.className = `tool-card tool-card-${t.status}`;

            const summary = document.createElement('summary');
            summary.className = 'tool-card-summary';
            const title = document.createElement('span');
            title.className = 'tool-card-title';
            title.textContent = t.name + (t.serverName ? ` (${t.serverName})` : '');
            const meta = document.createElement('span');
            meta.className = 'tool-card-meta';
            meta.textContent = t.status === 'running' ? '执行中…' : (t.status === 'failed' ? '失败' : '完成');
            summary.appendChild(title);
            summary.appendChild(meta);
            details.appendChild(summary);

            if (t.log) {
                const body = document.createElement('pre');
                body.className = t.status === 'failed'
                    ? 'tool-card-body tool-card-body-error'
                    : 'tool-card-body';
                body.textContent = t.log;
                details.appendChild(body);
            }
            return details;
        }

        function renderFeed() {
            feedEl.innerHTML = '';
            (state.feed || []).forEach((entry) => {
                if (entry.kind === 'tool') {
                    const t = findTool(entry.toolKey) || state.tools[entry.toolKey];
                    if (t) feedEl.appendChild(createToolCardEl(t));
                    return;
                }
                const row = document.createElement('div');
                const kind = entry.kind || 'turn';
                row.className = `timeline-entry timeline-${kind}`;
                row.innerHTML = `<span class="timeline-time">${escapeHtml(entry.time || '')}</span><span class="timeline-text">${escapeHtml(entry.summary || '')}</span>`;
                feedEl.appendChild(row);
            });
            feedEl.scrollTop = feedEl.scrollHeight;
        }

        function pushFeed(entry) {
            state.feed.push(entry);
            renderFeed();
        }

        function ensureFeedTool(key) {
            const exists = state.feed.some((e) => e.kind === 'tool' && e.toolKey === key);
            if (!exists) {
                state.feed.push({ kind: 'tool', toolKey: key });
            }
        }

        function getOrCreateTool(toolUseId, data) {
            const key = toolStorageKey(toolUseId);
            let t = state.tools[key];
            if (!t) {
                t = {
                    id: toolUseId,
                    name: data.tool_name || 'tool',
                    serverName: data.server_name || '',
                    status: 'running',
                    log: '',
                };
                state.tools[key] = t;
                ensureFeedTool(key);
            } else if (data.tool_name && t.name === 'tool') {
                t.name = data.tool_name;
            }
            if (data.server_name) t.serverName = data.server_name;
            return t;
        }

        function apply(rawMsg) {
            const msg = normalizeStreamMsg(rawMsg);
            if (!msg || !msg.type) return;

            if (msg.session_id) state.sessionId = msg.session_id;

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
                    const time = new Date().toLocaleTimeString('zh-CN', { hour12: false });
                    pushFeed({ kind: 'turn', time, summary: data.summary });
                }
                return;
            }

            const toolUseId = msg.tool_use_id;
            if (!toolUseId) return;

            if (data.type === 'mcp_progress') {
                let t = findTool(toolUseId);
                if (!t) {
                    t = getOrCreateTool(toolUseId, {
                        tool_name: data.tool_name || 'tool',
                        server_name: data.server_name,
                    });
                }
                if (data.server_name) t.serverName = data.server_name;
                if (data.status === 'failed') t.status = 'failed';
                else if (data.status === 'completed') t.status = 'success';
                else if (data.status === 'started') t.status = 'running';
                renderFeed();
                return;
            }

            const isToolProgress = data.type === 'tool_progress'
                || (data.status && data.type !== 'turn_progress' && data.type !== 'mcp_progress');
            if (!isToolProgress) return;

            const t = getOrCreateTool(toolUseId, data);
            if (data.status === 'started') {
                t.status = 'running';
                if (data.message) appendToolLog(t, data.message);
            } else if (data.status === 'completed' || data.status === 'failed') {
                t.status = data.status === 'failed' ? 'failed' : 'success';
                if (data.message) {
                    const max = data.status === 'failed' ? OUTPUT_PREVIEW_MAX : LOG_PREVIEW_MAX;
                    appendToolLog(t, data.message, max);
                }
            }
            renderFeed();
        }

        function applyStreamEvent(msg) {
            state.status = 'streaming';
            const ev = msg.event || {};
            const delta = ev.delta || {};
            if (delta.type === 'thinking_delta' && delta.thinking) {
                state.thinkingText += delta.thinking;
                thinkingBody.textContent = state.thinkingText;
                thinkingWrap.open = true;
            }
            if (delta.type === 'text_delta' && delta.text) {
                state.assistantText += delta.text;
                scheduleMarkdown();
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
            (m.tool_calls || []).forEach((tc) => {
                const id = pick(tc, 'id', 'Id', 'ID');
                const name = pick(tc, 'name', 'Name') || 'tool';
                const key = toolStorageKey(id);
                if (!state.tools[key]) {
                    state.tools[key] = {
                        id,
                        name,
                        status: 'running',
                        log: '',
                    };
                }
                ensureFeedTool(key);
            });
            scheduleMarkdown();
            renderFeed();
        }

        function applyResult(msg) {
            state.status = msg.is_error ? 'error' : 'done';
            state.result = {
                subtype: msg.subtype,
                stopReason: msg.stop_reason,
                numTurns: msg.num_turns,
                output: msg.output,
                error: msg.error,
            };
            if (msg.output) {
                state.assistantText = msg.output;
                scheduleMarkdown();
            }
            const label = msg.is_error
                ? `结束: ${msg.error || msg.stop_reason || 'error'}`
                : `完成 (${msg.num_turns || '?'} 轮, ${msg.stop_reason || 'ok'})`;
            const time = new Date().toLocaleTimeString('zh-CN', { hour12: false });
            pushFeed({
                kind: msg.is_error ? 'error' : 'done',
                time,
                summary: label,
            });
        }

        function reset() {
            Object.assign(state, emptyState());
            phaseEl.textContent = '';
            thinkingBody.textContent = '';
            assistantEl.innerHTML = '';
            feedEl.innerHTML = '';
            state.status = 'streaming';
            renderPhase();
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
            }));
        }

        function loadSnapshot(snap) {
            if (!snap) {
                reset();
                return;
            }
            Object.assign(state, emptyState(), snap);
            Object.values(state.tools).forEach(migrateToolFields);
            if (!state.feed || !state.feed.length) {
                state.feed = buildFeedFromLegacy(snap);
            }
            thinkingBody.textContent = state.thinkingText || '';
            thinkingWrap.open = !!state.thinkingText;
            scheduleMarkdown();
            renderFeed();
            renderPhase();
        }

        return { apply, reset, getSnapshot, loadSnapshot, getState: () => state };
    }

    global.SessionView = { create: createSessionView, emptyState };
})(window);
