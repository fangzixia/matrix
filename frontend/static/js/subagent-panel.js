/**
 * SubAgentPanel — 后台 Worker 任务列表、进度与停止（P1/P3）
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

    function createPanel(rootEl) {
        if (!rootEl) {
            return { upsert() {}, remove() {}, clear() {}, getAgents: () => ({}) };
        }

        const existing = rootEl.querySelector('.subagent-panel');
        if (existing) {
            const listEl = existing.querySelector('[data-list]');
            const countEl = existing.querySelector('[data-count]');
            const agents = new Map();
            return buildApi(existing, listEl, countEl, agents);
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
        return buildApi(panel, listEl, countEl, agents);
    }

    function buildApi(panel, listEl, countEl, agents) {
        function render() {
            const items = [...agents.values()].sort((a, b) => (b.created_at || 0) - (a.created_at || 0));
            const running = items.filter((a) => a.status === 'running').length;
            countEl.textContent = running > 0 ? `${running} 运行中` : `${items.length} 个`;

            if (items.length === 0) {
                listEl.innerHTML = '<div class="subagent-empty">暂无子 Agent</div>';
                panel.classList.add('subagent-panel-empty');
                return;
            }
            panel.classList.remove('subagent-panel-empty');

            listEl.innerHTML = items.map((a) => {
                const prog = a.progress || {};
                const activity = prog.last_activity || prog.summary || prog.current_tool || '';
                const turn = prog.turn > 0 ? `第 ${prog.turn} 轮` : '';
                const canStop = a.status === 'running';
                const parent = a.parent_agent_id ? `<span class="subagent-parent">↳ ${escapeHtml(a.parent_agent_id)}</span>` : '';
                return `
                    <div class="subagent-card subagent-${a.status}" data-id="${escapeHtml(a.id)}">
                        <div class="subagent-card-head">
                            <span class="subagent-id">${escapeHtml(a.id)}</span>
                            <span class="subagent-status">${escapeHtml(STATUS_LABEL[a.status] || a.status)}</span>
                        </div>
                        <div class="subagent-desc">${escapeHtml(a.description || '')}</div>
                        ${parent}
                        <div class="subagent-meta">${escapeHtml(turn)} ${activity ? ' · ' + escapeHtml(activity) : ''}</div>
                        <div class="subagent-actions">
                            ${canStop ? `<button type="button" class="btn btn-secondary btn-sm" data-stop="${escapeHtml(a.id)}">停止</button>` : ''}
                            <button type="button" class="btn btn-secondary btn-sm" data-expand="${escapeHtml(a.id)}">日志</button>
                        </div>
                        <pre class="subagent-transcript" data-transcript="${escapeHtml(a.id)}" hidden></pre>
                    </div>
                `;
            }).join('');

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

    global.SubAgentPanel = { create: createPanel };
})(window);
