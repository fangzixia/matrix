/**
 * SubAgentPanel — 后台 Worker 任务列表、进度与停止
 * 与 SessionView 联动：跳转 Worker 时间轴、展示摘要与统计
 */
(function (global) {
    const STATUS_LABEL = {
        running: '运行中',
        completed: '已完成',
        failed: '失败',
        stopped: '已停止',
    };

    function escapeHtml(s) {
        const d = document.createElement('div');
        d.textContent = s || '';
        return d.innerHTML;
    }

    function formatElapsed(ms) {
        const n = Number(ms) || 0;
        if (n <= 0) return '';
        if (n < 1000) return `${n}ms`;
        return `${(n / 1000).toFixed(1)}s`;
    }

    function createPanel(rootEl, options = {}) {
        const onJumpToWorker = options.onJumpToWorker
            || ((id) => global.SessionViewScrollToWorker?.(id));

        if (!rootEl) {
            return { upsert() {}, remove() {}, clear() {}, getAgents: () => ({}) };
        }

        const existing = rootEl.querySelector('.subagent-panel');
        if (existing) {
            const listEl = existing.querySelector('[data-list]');
            const countEl = existing.querySelector('[data-count]');
            const agents = new Map();
            return buildApi(rootEl, existing, listEl, countEl, agents, onJumpToWorker);
        }

        const panel = document.createElement('div');
        panel.className = 'subagent-panel';
        panel.innerHTML = `
            <div class="subagent-panel-header">
                <span class="subagent-panel-title">后台 Worker</span>
                <span class="subagent-panel-count" data-count>0</span>
            </div>
            <div class="subagent-list" data-list></div>
        `;
        rootEl.prepend(panel);

        const listEl = panel.querySelector('[data-list]');
        const countEl = panel.querySelector('[data-count]');
        const agents = new Map();
        return buildApi(rootEl, panel, listEl, countEl, agents, onJumpToWorker);
    }

    function buildApi(rootEl, panel, listEl, countEl, agents, onJumpToWorker) {
        function syncRootVisibility(items) {
            if (rootEl) {
                rootEl.classList.toggle('has-agents', items.length > 0);
            }
            if (typeof global.syncChatLiveStrip === 'function') {
                global.syncChatLiveStrip();
            }
        }

        function render() {
            const items = [...agents.values()].sort((a, b) => (b.created_at || 0) - (a.created_at || 0));
            const running = items.filter((a) => a.status === 'running').length;
            countEl.textContent = running > 0 ? `${running} 运行中` : `${items.length} 个`;

            if (items.length === 0) {
                listEl.innerHTML = '';
                panel.classList.add('subagent-panel-empty');
                syncRootVisibility(items);
                return;
            }
            panel.classList.remove('subagent-panel-empty');

            listEl.innerHTML = items.map((a) => {
                const prog = a.progress || {};
                const activity = prog.last_activity || prog.summary || prog.current_tool || '';
                const turn = prog.turn > 0 ? `第 ${prog.turn} 轮` : '';
                const toolCount = prog.tool_use_count > 0 ? ` · ${prog.tool_use_count} 次工具` : '';
                const tokens = (prog.input_tokens || prog.output_tokens)
                    ? ` · tokens ${prog.input_tokens || 0}/${prog.output_tokens || 0}`
                    : '';
                const canStop = a.status === 'running';
                const isRunning = a.status === 'running';
                const parent = a.parent_agent_id
                    ? `<span class="subagent-parent">↳ ${escapeHtml(a.parent_agent_id)}</span>`
                    : '';
                const preview = a.answer_preview
                    ? `<div class="subagent-answer-preview">${escapeHtml(truncatePreview(a.answer_preview, 160))}</div>`
                    : '';
                const turnCount = a.turn_count > 0 ? ` · 共 ${a.turn_count} 轮` : '';
                return `
                    <div class="subagent-card subagent-${a.status} ${isRunning ? 'subagent-card-active' : ''}" data-id="${escapeHtml(a.id)}">
                        <div class="subagent-card-head">
                            <span class="subagent-id">${escapeHtml(a.id)}</span>
                            <span class="subagent-status">${isRunning ? '<span class="tool-spinner"></span> ' : ''}${escapeHtml(STATUS_LABEL[a.status] || a.status)}</span>
                        </div>
                        <div class="subagent-desc">${escapeHtml(a.description || '')}</div>
                        ${parent}
                        <div class="subagent-meta">${escapeHtml(turn)}${escapeHtml(toolCount)}${escapeHtml(turnCount)}${activity ? ` · ${escapeHtml(activity)}` : ''}${escapeHtml(tokens)}</div>
                        ${preview}
                        <div class="subagent-actions">
                            <button type="button" class="btn btn-secondary btn-sm" data-jump="${escapeHtml(a.id)}">查看步骤</button>
                            ${canStop ? `<button type="button" class="btn btn-secondary btn-sm" data-stop="${escapeHtml(a.id)}">停止</button>` : ''}
                            <button type="button" class="btn btn-secondary btn-sm" data-expand="${escapeHtml(a.id)}">日志</button>
                        </div>
                        <pre class="subagent-transcript" data-transcript="${escapeHtml(a.id)}" hidden></pre>
                    </div>
                `;
            }).join('');

            listEl.querySelectorAll('[data-jump]').forEach((btn) => {
                btn.addEventListener('click', (e) => {
                    e.preventDefault();
                    const id = btn.getAttribute('data-jump');
                    if (id && onJumpToWorker) onJumpToWorker(id);
                });
            });

            listEl.querySelectorAll('[data-stop]').forEach((btn) => {
                btn.addEventListener('click', async () => {
                    const id = btn.getAttribute('data-stop');
                    if (!id || !global.WailsAPI?.stopSubAgent) return;
                    btn.disabled = true;
                    try {
                        await global.WailsAPI.stopSubAgent(id, '用户从面板停止');
                    } catch (e) {
                        console.error(e);
                    } finally {
                        btn.disabled = false;
                    }
                });
            });

            listEl.querySelectorAll('[data-expand]').forEach((btn) => {
                btn.addEventListener('click', async () => {
                    const id = btn.getAttribute('data-expand');
                    const pre = listEl.querySelector(`[data-transcript="${id}"]`);
                    if (!pre || !global.WailsAPI?.readSubAgentTranscript) return;
                    if (!pre.hidden && pre.textContent) {
                        pre.hidden = true;
                        return;
                    }
                    pre.hidden = false;
                    pre.textContent = '加载中…';
                    try {
                        pre.textContent = await global.WailsAPI.readSubAgentTranscript(id, 80);
                    } catch (e) {
                        pre.textContent = String(e);
                    }
                });
            });
            syncRootVisibility(items);
        }

        return {
            upsert(snap) {
                if (!snap || !snap.id) return;
                agents.set(snap.id, snap);
                render();
            },
            remove(id) {
                agents.delete(id);
                render();
            },
            clear() {
                agents.clear();
                render();
            },
            getAgents() {
                return Object.fromEntries(agents);
            },
        };
    }

    function truncatePreview(s, max) {
        s = String(s || '');
        return s.length <= max ? s : s.slice(0, max) + '…';
    }

    global.SubAgentPanel = { create: createPanel };
})(window);
