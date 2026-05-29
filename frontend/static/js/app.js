console.log('[app.js] Loaded - Version: 2026-05-20-00-35');

let executionSessionView = null;
let executionSubAgentPanel = null;

/** 自由对话与执行任务共用：同一 SessionView 行为，仅挂载容器不同 */
function createSubAgentPanel(rootEl, getSessionView) {
    if (!rootEl || !window.SubAgentPanel) return null;
    return window.SubAgentPanel.create(rootEl, {
        onJumpToWorker: (id) => getSessionView()?.scrollToWorker?.(id),
    });
}

function getExecutionSubAgentPanel() {
    const root = $('#subagent-panel-root');
    if (!root) return null;
    if (!executionSubAgentPanel) {
        executionSubAgentPanel = createSubAgentPanel(root, getExecutionSessionView);
    }
    return executionSubAgentPanel;
}

function getExecutionSessionView() {
    const root = $('#session-view-root');
    if (!root || !window.SessionView) return null;
    if (!executionSessionView) {
        executionSessionView = window.SessionView.create(root, {
            subagentPanel: getExecutionSubAgentPanel(),
        });
    }
    return executionSessionView;
}

function subAgentStreamHooks(panel) {
    if (!panel) return {};
    return {
        onSubAgentUpdate: (snap) => panel.upsert(snap),
        onSubAgentDone: (snap) => panel.upsert(snap),
    };
}

function getExecutionSlot(action) {
    if (!action) return createEmptyExecutionSlot();
    if (!state.executionByPersona[action]) {
        state.executionByPersona[action] = createEmptyExecutionSlot();
    }
    return state.executionByPersona[action];
}

function syncExecutionPageLayout() {
    const page = $('#execution-page');
    const progressEl = $('#execution-progress');
    if (!page || !progressEl) return;
    const live = progressEl.style.display !== 'none';
    page.classList.toggle('execution-page--live', live);
}

/** 切换「执行任务」页内不同卡片时，恢复该卡片对应的进度/结果 UI */
function applyExecutionViewForPersona(action) {
    const slot = getExecutionSlot(action);
    const progressEl = $('#execution-progress');
    const resultEl = $('#execution-result');
    const view = getExecutionSessionView();

    if (slot.running || slot.sessionSnapshot || slot.result) {
        progressEl.style.display = 'block';
        if (view) {
            view.reset();
            if (slot.sessionSnapshot) view.loadSnapshot(slot.sessionSnapshot);
        }
        if (slot.result) {
            renderResultToDOM(slot);
            resultEl.style.display = 'block';
        } else {
            resultEl.style.display = 'none';
        }
    } else {
        progressEl.style.display = 'none';
        resultEl.style.display = 'none';
    }
    syncExecutionPageLayout();
}

function renderResultToDOM(slot) {
    const statusEl = $('#result-status');
    const contentEl = $('#result-content');
    contentEl.innerHTML = '';

    if (slot.hasError && slot.result) {
        statusEl.textContent = '执行失败';
        statusEl.className = 'result-status error';
        const errText = slot.result.error || '未知错误';
        const pre = document.createElement('pre');
        pre.style.whiteSpace = 'pre-wrap';
        pre.textContent = errText;
        contentEl.appendChild(pre);
        return;
    }

    if (!slot.result) {
        statusEl.textContent = '';
        statusEl.className = 'result-status';
        return;
    }

    const r = slot.result;
    statusEl.textContent = r.has_error ? '执行完成（有错误）' : '执行成功';
    statusEl.className = `result-status ${r.has_error ? 'error' : 'success'}`;
    const pre = document.createElement('pre');
    pre.style.whiteSpace = 'pre-wrap';
    pre.textContent = r.output || '无输出';
    contentEl.appendChild(pre);

    if (!r.has_error && r.output) {
        const fileMatch = r.output.match(/\.spec\/[^\s]+\.md/g);
        if (fileMatch && fileMatch.length > 0) {
            const filePath = fileMatch[fileMatch.length - 1];
            const viewBtn = document.createElement('button');
            viewBtn.className = 'btn btn-secondary';
            viewBtn.innerHTML = '<span class="btn-icon">👁️</span> 查看生成的文件';
            viewBtn.style.marginTop = '12px';
            viewBtn.onclick = async () => {
                try {
                    const response = await api.readFile(filePath);
                    openFileEditor(filePath, response.content);
                } catch (error) {
                    showNotification('加载文件失败: ' + error.message, 'error');
                }
            };
            contentEl.appendChild(viewBtn);
        }
    }
}

const formatDuration = (ms) => {
    const seconds = Math.floor(ms / 1000);
    const minutes = Math.floor(seconds / 60);
    const hours = Math.floor(minutes / 60);
    
    if (hours > 0) {
        return `${hours}:${String(minutes % 60).padStart(2, '0')}:${String(seconds % 60).padStart(2, '0')}`;
    }
    return `${String(minutes).padStart(2, '0')}:${String(seconds % 60).padStart(2, '0')}`;
};

// API 调用 - 使用 WailsAPI 适配层
const api = {
    async call(endpoint, options = {}) {
        // 对于通用的 API 调用，根据端点路由到相应的 WailsAPI 方法
        try {
            if (endpoint === '/v1/requirements') {
                return await WailsAPI.getRequirements();
            } else if (endpoint === '/v1/evaluations') {
                return await WailsAPI.getEvaluations();
            } else if (endpoint === '/v1/config') {
                return await WailsAPI.getConfig();
            } else if (endpoint === '/v1/config/env') {
                if (options.method === 'POST') {
                    return await WailsAPI.saveEnvConfig(JSON.parse(options.body));
                } else {
                    return await WailsAPI.getEnvConfig();
                }
            } else if (endpoint.startsWith('/v1/files/list')) {
                const url = new URL(endpoint, 'http://dummy');
                const path = url.searchParams.get('path') || '';
                return await WailsAPI.listFiles(path);
            } else if (endpoint.startsWith('/v1/files/read')) {
                const url = new URL(endpoint, 'http://dummy');
                const path = url.searchParams.get('path') || '';
                return await WailsAPI.readFile(path);
            } else if (endpoint === '/v1/files/save') {
                const body = JSON.parse(options.body);
                return await WailsAPI.saveFile(body.path, body.content);
            } else {
                throw new Error(`Unsupported endpoint: ${endpoint}`);
            }
        } catch (error) {
            console.error('API Error:', error);
            throw error;
        }
    },

    async runPersona(personaID, task, filePath = '') {
        return await WailsAPI.runTaskSession(personaID, task, filePath);
    },

    async runPersonaStreaming(personaID, task, filePath = '', onStream, onDone, onError) {
        return await WailsAPI.runPersonaStreaming(personaID, task, filePath, onStream, onDone, onError);
    },

    async runBuild(task, filePath = '', onStream, onDone, onError) {
        return await WailsAPI.runPersonaStreaming('build', task, filePath, onStream, onDone, onError);
    },

    async cancelAgentSession() {
        return await WailsAPI.cancelAgentSession();
    },

    async getFiles(path) {
        return await WailsAPI.listFiles(path);
    },

    async readFile(path) {
        return await WailsAPI.readFile(path);
    },

    async saveFile(path, content) {
        return await WailsAPI.saveFile(path, content);
    },
};

// 页面导航
function initNavigation() {
    $$('.nav-link').forEach(link => {
        link.addEventListener('click', (e) => {
            e.preventDefault();
            const page = link.dataset.page;
            showPage(page);
        });
    });
}

function showPage(pageName) {
    // 更新导航状态
    $$('.nav-link').forEach(link => {
        link.classList.toggle('active', link.dataset.page === pageName);
    });

    // 显示对应页面
    $$('.page').forEach(page => {
        page.classList.toggle('active', page.id === `${pageName}-page`);
    });

    // 加载页面数据
    loadPageData(pageName);
}

async function loadPageData(pageName) {
    switch (pageName) {
        case 'dashboard':
            await loadDashboard();
            break;
        case 'chat':
            ChatPage.onShow();
            break;
        case 'execution':
            if (state.currentPersona) {
                applyExecutionViewForPersona(state.currentPersona);
            } else {
                $('#execution-progress').style.display = 'none';
                $('#execution-result').style.display = 'none';
            }
            break;
        case 'history':
            await loadHistory();
            break;
    }
}

// 仪表盘
async function loadDashboard() {
    try {
        // 加载需求数据
        await loadRequirementsData();
        
        // 更新统计
        updateStats();
        
        // 显示最近需求
        renderRecentRequirements();
        
        // 显示失败项统计
        renderFailureStats();
    } catch (error) {
        console.error('加载仪表盘失败:', error);
    }
}

function updateStats() {
    // 确保 requirements 和 evaluations 是数组
    const requirements = Array.isArray(state.requirements) ? state.requirements : [];
    const evaluations = Array.isArray(state.evaluations) ? state.evaluations : [];
    
    const total = requirements.length;
    const evalCount = evaluations.length;
    const passRate = total > 0 ? Math.round((evalCount / total) * 100) : 0;

    $('#total-requirements').textContent = total;
    $('#passed-requirements').textContent = evalCount;
    $('#failed-requirements').textContent = Math.max(0, total - evalCount);
    $('#pass-rate').textContent = `${passRate}%`;
}

function renderRecentRequirements() {
    const container = $('#recent-requirements');
    // 确保 requirements 是数组
    const requirements = Array.isArray(state.requirements) ? state.requirements : [];
    const recent = requirements.slice(0, 5);

    if (recent.length === 0) {
        container.innerHTML = '<div class="empty-state">暂无需求数据</div>';
        return;
    }

    container.innerHTML = recent.map(req => `
        <div class="requirement-item" data-id="${req.id}">
            <div class="requirement-header">
                <span class="requirement-id">${req.id}</span>
            </div>
            <div class="requirement-meta">
                <span>📄 ${req.path}</span>
            </div>
        </div>
    `).join('');

    // 绑定点击事件
    container.querySelectorAll('.requirement-item').forEach(item => {
        item.addEventListener('click', () => {
            const id = item.dataset.id;
            showRequirementDetail(id);
        });
    });
}

function renderFailureStats() {
    const container = $('#failure-stats');
    
    // 统计失败项类别
    const failureCategories = {};
    state.evaluations.forEach(eval => {
        if (eval.failedItems) {
            eval.failedItems.forEach(item => {
                const category = item.category || '其他';
                failureCategories[category] = (failureCategories[category] || 0) + 1;
            });
        }
    });

    const categories = Object.entries(failureCategories);
    
    if (categories.length === 0) {
        container.innerHTML = '<div class="empty-state">暂无失败项数据</div>';
        return;
    }

    container.innerHTML = categories.map(([category, count]) => `
        <div class="failure-item">
            <div class="failure-category">${getCategoryText(category)}</div>
            <div class="failure-count">${count} 项</div>
        </div>
    `).join('');
}

// 需求管理
async function loadRequirements() {
    await loadRequirementsData();
    renderRequirementsList();
}

async function loadRequirementsData() {
    try {
        // 加载真实需求数据
        const reqResponse = await WailsAPI.getRequirements();
        state.requirements = reqResponse.requirements || [];
        
        // 加载真实验收数据
        const evalResponse = await WailsAPI.getEvaluations();
        state.evaluations = evalResponse.evaluations || [];
    } catch (error) {
        console.error('加载需求数据失败:', error);
        state.requirements = [];
        state.evaluations = [];
    }
}

function renderRequirementsList() {
    const container = $('#requirements-list');

    if (state.requirements.length === 0) {
        container.innerHTML = '<div class="empty-state">暂无需求数据</div>';
        return;
    }

    container.innerHTML = state.requirements.map(req => `
        <div class="requirement-item" data-id="${req.id}">
            <div class="requirement-header">
                <span class="requirement-id">${req.id}</span>
            </div>
            <div class="requirement-meta">
                <span>� ${req.path}</span>
            </div>
        </div>
    `).join('');

    // 绑定点击事件
    container.querySelectorAll('.requirement-item').forEach(item => {
        item.addEventListener('click', () => {
            const id = item.dataset.id;
            showRequirementDetail(id);
        });
    });
}

// 执行任务
function initExecution() {
    // 快速操作卡片
    $$('.action-card').forEach(card => {
        card.addEventListener('click', () => {
            const persona = card.dataset.persona;
            showExecutionForm(persona);
        });
    });

    // 表单按钮
    $('#close-execution-form').addEventListener('click', hideExecutionForm);
    $('#cancel-execution-btn').addEventListener('click', hideExecutionForm);
    $('#submit-execution-btn').addEventListener('click', submitExecution);
    $('#new-execution-btn').addEventListener('click', () => {
        hideExecutionResult();
        showPage('execution');
    });
}

async function showExecutionForm(persona) {
    state.currentPersona = persona;
    applyExecutionViewForPersona(persona);

    const titles = {
        spec: '创建需求',
        implement: '编码实现',
        verify: '验收评测',
        build: '完整构建',
        'ui-scan': '页面扫描',
        dialogue: '自由对话',
    };

    const placeholders = {
        spec: '请描述需求...',
        implement: '（可选）指定实现重点...',
        verify: '（可选）指定验收重点...',
        build: '（可选）指定构建参数...',
        'ui-scan': '请用自然语言描述，例如：访问 https://app.example.com ，用户名 admin 密码 ***，登录后遍历左侧菜单，记录路由、子 Tab 和按钮，跳过「日志」菜单…',
        dialogue: '请输入你的需求或问题，Agent 将直接执行...',
    };

    $('#execution-form-title').textContent = titles[persona] || '执行任务';
    $('#task-input').value = '';
    $('#task-input').placeholder = placeholders[persona] || '请输入任务描述...';

    // 除 spec / dialogue / ui-scan 外，任务描述均为可选
    const taskOptional = persona !== 'spec' && persona !== 'dialogue' && persona !== 'ui-scan';
    const label = $('#task-input').previousElementSibling;
    if (label && label.tagName === 'LABEL') {
        if (persona === 'ui-scan') {
            label.textContent = '扫描说明';
        } else {
            label.textContent = taskOptional ? '任务描述（可选）' : '任务描述';
        }
    }

    // 根据不同的 persona 显示不同的表单字段
    await setupFormFields(persona);

    const page = $('#execution-page');
    const quickActions = $('.quick-actions');
    if (page) page.classList.add('execution-page--form');
    if (quickActions) quickActions.style.display = 'none';

    $('#execution-form').style.display = 'block';
    const taskInput = $('#task-input');
    if (taskInput) {
        taskInput.rows = persona === 'ui-scan' ? 8 : 4;
    }
    $('#task-input').focus();

    requestAnimationFrame(() => {
        $('#submit-execution-btn')?.scrollIntoView({ block: 'nearest', behavior: 'smooth' });
    });

    $$('.action-card').forEach((c) => {
        c.classList.toggle('action-card--active', c.dataset.persona === persona);
    });
}


function hideExecutionForm() {
    $('#execution-form').style.display = 'none';
    const page = $('#execution-page');
    const quickActions = $('.quick-actions');
    if (page) page.classList.remove('execution-page--form');
    if (quickActions) quickActions.style.display = '';
}

// 设置表单字段
async function setupFormFields(persona) {
    // 清除之前的动态字段
    const existingDynamicFields = $('#execution-form .form-body').querySelectorAll('.dynamic-field');
    existingDynamicFields.forEach(field => field.remove());
    
    const formBody = $('#execution-form .form-body');
    const actionsDiv = formBody.querySelector('.form-actions');
    
    if (persona === 'spec') {
        // 创建需求：显示已有需求文件选择
        await addRequirementsFields(formBody, actionsDiv);
    } else if (persona === 'implement') {
        // 编码实现：需求文件下拉框
        await addCodeFields(formBody, actionsDiv);
    } else if (persona === 'verify') {
        // 验收评测：需求文件下拉框 + 评测文件下拉框（只读）
        await addEvalFields(formBody, actionsDiv);
    } else if (persona === 'build') {
        // 完整构建：需求文件下拉框
        await addBuildFields(formBody, actionsDiv);
    }
    // ui-scan / dialogue: 无额外动态字段
}

// 创建需求的字段
async function addRequirementsFields(formBody, actionsDiv) {
    try {
        // 确保加载需求数据
        if (state.requirements.length === 0) {
            await loadRequirementsData();
        }
        
        const fieldDiv = document.createElement('div');
        fieldDiv.className = 'form-group dynamic-field';
        
        if (state.requirements.length > 0) {
            fieldDiv.innerHTML = `
                <label for="existing-req-select">参考已有需求（可选）</label>
                <div class="select-with-action">
                    <select id="existing-req-select" class="form-control">
                        <option value="">-- 不参考 --</option>
                        ${state.requirements.map(req => 
                            `<option value="${req.path}">${req.id}</option>`
                        ).join('')}
                    </select>
                    <button type="button" class="btn btn-secondary btn-sm" id="view-req-file-btn" style="display: none;">
                        <span class="btn-icon">👁️</span>
                        查看/编辑
                    </button>
                </div>
            `;
        } else {
            fieldDiv.innerHTML = `
                <label>参考已有需求（可选）</label>
                <div class="form-text">暂无已有需求文件</div>
            `;
        }
        
        formBody.insertBefore(fieldDiv, actionsDiv);
        
        if (state.requirements.length > 0) {
            const selectEl = $('#existing-req-select');
            const viewBtn = $('#view-req-file-btn');
            
            selectEl.addEventListener('change', () => {
                viewBtn.style.display = selectEl.value ? 'inline-block' : 'none';
            });
            
            viewBtn.addEventListener('click', async () => {
                const path = selectEl.value;
                if (!path) return;
                
                try {
                    const response = await api.readFile(path);
                    openFileEditor(path, response.content, false);
                } catch (error) {
                    showNotification('加载文件失败: ' + error.message, 'error');
                }
            });
        }
    } catch (error) {
        console.error('加载需求列表失败:', error);
        // 显示错误提示
        const fieldDiv = document.createElement('div');
        fieldDiv.className = 'form-group dynamic-field';
        fieldDiv.innerHTML = `
            <label>参考已有需求（可选）</label>
            <div class="form-text" style="color: #ef4444;">加载需求列表失败</div>
        `;
        formBody.insertBefore(fieldDiv, actionsDiv);
    }
}

// 编码实现的字段
async function addCodeFields(formBody, actionsDiv) {
    try {
        // 确保加载需求数据
        if (state.requirements.length === 0) {
            await loadRequirementsData();
        }
        
        const fieldDiv = document.createElement('div');
        fieldDiv.className = 'form-group dynamic-field';
        fieldDiv.innerHTML = `
            <label for="code-req-select">需求文件路径（可选）</label>
            <div class="select-with-action">
                <select id="code-req-select" class="form-control">
                    <option value="">-- 使用最新需求 --</option>
                    ${state.requirements.map(req => 
                        `<option value="${req.path}">${req.id}</option>`
                    ).join('')}
                </select>
                <button type="button" class="btn btn-secondary btn-sm" id="view-code-req-btn" style="display: none;">
                    <span class="btn-icon">👁️</span>
                    查看/编辑
                </button>
            </div>
            <small class="form-text">${state.requirements.length > 0 ? '留空则使用最新需求文件' : '暂无需求文件，留空将使用最新生成的需求'}</small>
        `;
        formBody.insertBefore(fieldDiv, actionsDiv);
        
        const selectEl = $('#code-req-select');
        const viewBtn = $('#view-code-req-btn');
        
        selectEl.addEventListener('change', () => {
            viewBtn.style.display = selectEl.value ? 'inline-block' : 'none';
        });
        
        viewBtn.addEventListener('click', async () => {
            const path = selectEl.value;
            if (!path) return;
            
            try {
                const response = await api.readFile(path);
                openFileEditor(path, response.content, false);
            } catch (error) {
                showNotification('加载文件失败: ' + error.message, 'error');
            }
        });
    } catch (error) {
        console.error('加载需求列表失败:', error);
        // 显示错误提示
        const fieldDiv = document.createElement('div');
        fieldDiv.className = 'form-group dynamic-field';
        fieldDiv.innerHTML = `
            <label for="code-req-select">需求文件路径（可选）</label>
            <div class="form-text" style="color: #ef4444;">加载需求列表失败，留空将使用最新需求</div>
        `;
        formBody.insertBefore(fieldDiv, actionsDiv);
    }
}

// 验收评测的字段
async function addEvalFields(formBody, actionsDiv) {
    try {
        // 确保加载需求数据
        if (state.requirements.length === 0) {
            await loadRequirementsData();
        }
        
        // 需求文件下拉框
        const reqFieldDiv = document.createElement('div');
        reqFieldDiv.className = 'form-group dynamic-field';
        reqFieldDiv.innerHTML = `
            <label for="eval-req-select">需求文件路径（可选）</label>
            <div class="select-with-action">
                <select id="eval-req-select" class="form-control">
                    <option value="">-- 使用最新需求 --</option>
                    ${state.requirements.map(req => 
                        `<option value="${req.path}">${req.id}</option>`
                    ).join('')}
                </select>
                <button type="button" class="btn btn-secondary btn-sm" id="view-eval-req-btn" style="display: none;">
                    <span class="btn-icon">👁️</span>
                    查看
                </button>
            </div>
            <small class="form-text">${state.requirements.length > 0 ? '留空则使用最新需求文件' : '暂无需求文件，留空将使用最新生成的需求'}</small>
        `;
        formBody.insertBefore(reqFieldDiv, actionsDiv);
        
        // 评测文件下拉框
        const evalFieldDiv = document.createElement('div');
        evalFieldDiv.className = 'form-group dynamic-field';
        evalFieldDiv.innerHTML = `
            <label for="eval-file-select">查看历史评测（可选）</label>
            <div class="select-with-action">
                <select id="eval-file-select" class="form-control">
                    <option value="">-- 选择评测文件 --</option>
                    ${state.evaluations.map(ev => 
                        `<option value="${ev.path}">${ev.requirementId} - 第${ev.round}轮 (${ev.score}分)</option>`
                    ).join('')}
                </select>
                <button type="button" class="btn btn-secondary btn-sm" id="view-eval-file-btn" style="display: none;">
                    <span class="btn-icon">👁️</span>
                    查看
                </button>
            </div>
        `;
        formBody.insertBefore(evalFieldDiv, actionsDiv);
        
        // 需求文件查看按钮
        const reqSelectEl = $('#eval-req-select');
        const reqViewBtn = $('#view-eval-req-btn');
        
        reqSelectEl.addEventListener('change', () => {
            reqViewBtn.style.display = reqSelectEl.value ? 'inline-block' : 'none';
        });
        
        reqViewBtn.addEventListener('click', async () => {
            const path = reqSelectEl.value;
            if (!path) return;
            
            try {
                const response = await api.readFile(path);
                openFileEditor(path, response.content, true); // 只读
            } catch (error) {
                showNotification('加载文件失败: ' + error.message, 'error');
            }
        });
        
        // 评测文件查看按钮
        const evalSelectEl = $('#eval-file-select');
        const evalViewBtn = $('#view-eval-file-btn');
        
        evalSelectEl.addEventListener('change', () => {
            evalViewBtn.style.display = evalSelectEl.value ? 'inline-block' : 'none';
        });
        
        evalViewBtn.addEventListener('click', async () => {
            const path = evalSelectEl.value;
            if (!path) return;
            
            try {
                const response = await api.readFile(path);
                openFileEditor(path, response.content, true); // 只读
            } catch (error) {
                showNotification('加载文件失败: ' + error.message, 'error');
            }
        });
    } catch (error) {
        console.error('加载评测列表失败:', error);
    }
}

// 完整构建的字段
async function addBuildFields(formBody, actionsDiv) {
    try {
        // 确保加载需求数据
        if (state.requirements.length === 0) {
            await loadRequirementsData();
        }
        
        const fieldDiv = document.createElement('div');
        fieldDiv.className = 'form-group dynamic-field';
        fieldDiv.innerHTML = `
            <label for="build-req-select">需求文件路径（可选）</label>
            <div class="select-with-action">
                <select id="build-req-select" class="form-control">
                    <option value="">-- 使用最新需求 --</option>
                    ${state.requirements.map(req => 
                        `<option value="${req.path}">${req.id}</option>`
                    ).join('')}
                </select>
                <button type="button" class="btn btn-secondary btn-sm" id="view-build-req-btn" style="display: none;">
                    <span class="btn-icon">👁️</span>
                    查看/编辑
                </button>
            </div>
            <small class="form-text">${state.requirements.length > 0 ? '留空则使用最新需求文件' : '暂无需求文件，留空将使用最新生成的需求'}</small>
        `;
        formBody.insertBefore(fieldDiv, actionsDiv);
        
        const selectEl = $('#build-req-select');
        const viewBtn = $('#view-build-req-btn');
        
        selectEl.addEventListener('change', () => {
            viewBtn.style.display = selectEl.value ? 'inline-block' : 'none';
        });
        
        viewBtn.addEventListener('click', async () => {
            const path = selectEl.value;
            if (!path) return;
            
            try {
                const response = await api.readFile(path);
                openFileEditor(path, response.content, false);
            } catch (error) {
                showNotification('加载文件失败: ' + error.message, 'error');
            }
        });
    } catch (error) {
        console.error('加载需求列表失败:', error);
        // 显示错误提示
        const fieldDiv = document.createElement('div');
        fieldDiv.className = 'form-group dynamic-field';
        fieldDiv.innerHTML = `
            <label for="build-req-select">需求文件路径（可选）</label>
            <div class="form-text" style="color: #ef4444;">加载需求列表失败，留空将使用最新需求</div>
        `;
        formBody.insertBefore(fieldDiv, actionsDiv);
    }
}

async function submitExecution() {
    const task = $('#task-input').value.trim();
    let filePath = '';

    if (state.currentPersona === 'ui-scan' && !task) {
        showNotification('请用自然语言描述扫描目标、访问地址、登录方式与范围', 'error');
        return;
    }

    // 根据不同的 persona 获取需求路径
    if (state.currentPersona === 'spec') {
        const selectEl = $('#existing-req-select');
        filePath = selectEl ? selectEl.value : '';
    } else if (state.currentPersona === 'implement') {
        const selectEl = $('#code-req-select');
        filePath = selectEl ? selectEl.value : '';
    } else if (state.currentPersona === 'verify') {
        const selectEl = $('#eval-req-select');
        filePath = selectEl ? selectEl.value : '';
    } else if (state.currentPersona === 'build') {
        const selectEl = $('#build-req-select');
        filePath = selectEl ? selectEl.value : '';
    }
    
    hideExecutionForm();
    showExecutionProgress();

    console.log('[App] Starting execution:', state.currentPersona, task, filePath);

    const action = state.currentPersona;
    const view = getExecutionSessionView();
    const stopBtn = $('#stop-execution-btn');
    if (stopBtn) {
        stopBtn.disabled = false;
        stopBtn.onclick = () => api.cancelAgentSession();
    }

    try {
        const panel = getExecutionSubAgentPanel();
        await api.runPersonaStreaming(
            action,
            task,
            filePath,
            (msg) => {
                const slot = getExecutionSlot(action);
                if (state.currentPersona === action && view) {
                    view.apply(msg);
                }
                if (view) slot.sessionSnapshot = view.getSnapshot();
            },
            (result) => {
                if (stopBtn) stopBtn.disabled = true;
                showExecutionResult(result, false);
            },
            (error) => {
                if (stopBtn) stopBtn.disabled = true;
                showExecutionResult(error, true);
            },
            subAgentStreamHooks(panel)
        );
    } catch (error) {
        if (stopBtn) stopBtn.disabled = true;
        showExecutionResult({ error: error.message }, true);
    }
}

function showExecutionProgress() {
    state.streamingPersona = state.currentPersona;
    const action = state.streamingPersona;
    const slot = getExecutionSlot(action);
    slot.running = true;
    slot.result = null;
    slot.hasError = false;
    slot.sessionSnapshot = null;

    $('#execution-progress').style.display = 'block';
    $('#execution-result').style.display = 'none';
    syncExecutionPageLayout();

    const view = getExecutionSessionView();
    if (view) view.reset();
    const panel = getExecutionSubAgentPanel();
    if (panel && api.listSubAgents) {
        api.listSubAgents().then((list) => {
            (list || []).forEach((s) => panel.upsert(s));
        }).catch(() => {});
    }
}

function hideExecutionProgress() {
    $('#execution-progress').style.display = 'none';
    syncExecutionPageLayout();
    const stopBtn = $('#stop-execution-btn');
    if (stopBtn) stopBtn.disabled = true;
}

function showExecutionResult(result, hasError) {
    const action = state.streamingPersona || state.currentPersona;
    const slot = getExecutionSlot(action);
    const view = getExecutionSessionView();
    slot.running = false;
    state.streamingPersona = null;

    if (view) {
        view.finalizeRunningTools(hasError ? 'error' : 'done');
        slot.sessionSnapshot = view.getSnapshot();
    }

    if (hasError) {
        slot.hasError = true;
        slot.result = { error: result.error || result.message || '未知错误' };
    } else {
        slot.hasError = false;
        slot.result = {
            output: result.output,
            has_error: result.has_error,
        };
    }

    const stopBtn = $('#stop-execution-btn');
    if (stopBtn) stopBtn.disabled = true;

    if (state.currentPersona === action) {
        if (view && slot.sessionSnapshot) view.loadSnapshot(slot.sessionSnapshot);
        renderResultToDOM(slot);
        $('#execution-result').style.display = 'block';
    }

    loadRequirementsData();
}

function hideExecutionResult() {
    $('#execution-result').style.display = 'none';
}

// 验收历史
async function loadHistory() {
    await loadRequirementsData();
    renderHistoryList();
    updateHistoryFilters();
}

function updateHistoryFilters() {
    const filterReq = $('#filter-requirement');
    filterReq.innerHTML = '<option value="">全部需求</option>' +
        state.requirements.map(req => `<option value="${req.id}">${req.id}</option>`).join('');
}

function renderHistoryList() {
    const container = $('#history-list');
    
    if (state.evaluations.length === 0) {
        container.innerHTML = '<div class="empty-state">暂无验收历史</div>';
        return;
    }

    container.innerHTML = state.evaluations.map(eval => {
        const parts = eval.id.split('-');
        const round = parts[parts.length - 1] || '?';
        return `
        <div class="history-item" data-path="${eval.path}">
            <div class="history-header">
                <span class="history-title">${eval.requirementId} - 第 ${round} 轮验收</span>
            </div>
            <div class="history-meta">
                <span>📄 ${eval.path}</span>
            </div>
            <div class="history-actions">
                <button class="btn btn-secondary btn-sm view-eval-btn" data-path="${eval.path}">
                    <span class="btn-icon">👁️</span>
                    查看详情
                </button>
            </div>
        </div>`;
    }).join('');

    // 绑定查看按钮
    container.querySelectorAll('.view-eval-btn').forEach(btn => {
        btn.addEventListener('click', async (e) => {
            e.stopPropagation();
            const path = btn.dataset.path;
            try {
                const response = await api.readFile(path);
                openFileEditor(path, response.content);
            } catch (error) {
                showNotification('加载文件失败: ' + error.message, 'error');
            }
        });
    });
}

// 模态框
function initModals() {
    // 需求详情模态框
    $('#close-requirement-detail-modal').addEventListener('click', () => {
        hideModal('requirement-detail-modal');
    });
    
    $('#close-requirement-detail').addEventListener('click', () => {
        hideModal('requirement-detail-modal');
    });

    $('#edit-requirement-detail').addEventListener('click', () => {
        if (state.currentRequirement) {
            hideModal('requirement-detail-modal');
            openFileEditor(state.currentRequirement.path, state.currentRequirement.content);
        }
    });
    
    $('#run-build-from-detail').addEventListener('click', () => {
        hideModal('requirement-detail-modal');
        showPage('execution');
        showExecutionForm('build');
        if (state.currentRequirement) {
            $('#requirements-path-input').value = state.currentRequirement.path || '';
        }
    });

    // 文件编辑器模态框
    initFileEditor();
}

function showModal(modalId) {
    $(`#${modalId}`).classList.add('active');
}

function hideModal(modalId) {
    $(`#${modalId}`).classList.remove('active');
}

function showRequirementDetail(reqId) {
    const req = state.requirements.find(r => r.id === reqId);
    if (!req) return;
    
    state.currentRequirement = req;

    $('#requirement-detail-title').textContent = req.id;
    $('#requirement-detail-content').innerHTML = '<div class="empty-state">加载中...</div>';

    showModal('requirement-detail-modal');

    api.readFile(req.path).then(res => {
        $('#requirement-detail-content').innerHTML = formatMarkdown(res.content || '暂无内容');
    }).catch(err => {
        $('#requirement-detail-content').innerHTML = `<div class="error-message">加载失败: ${err.message}</div>`;
    });
}

// 工具函数
function getStatusText(status) {
    const texts = {
        passed: '已通过',
        failed: '未通过',
        pending: '待验收',
    };
    return texts[status] || status;
}

function getCategoryText(category) {
    const texts = {
        blocking: '🚫 阻塞性失败',
        contract: '🔗 契约不一致',
        ux: '👤 用户体验问题',
        edge_case: '🔍 边缘情况',
        other: '📌 其他',
    };
    return texts[category] || category;
}

/** 文档页等场景：保留适度段落间距 */
function normalizeMarkdownText(text) {
    if (!text) return '';
    return text
        .replace(/\r\n/g, '\n')
        .replace(/[ \t]+\n/g, '\n')
        .replace(/\n{3,}/g, '\n\n')
        .trim();
}

/**
 * 对话/进度区：收紧源文本空行，避免列表项之间被拆成多个段落块。
 */
function normalizeChatMarkdown(text) {
    if (!text) return '';
    let s = normalizeMarkdownText(text);
    // 列表项前不要空段落（常见模型输出：\n\n- item）
    s = s.replace(/\n{2,}(?=[-*+]\s)/g, '\n');
    s = s.replace(/\n{2,}(?=\d+\.\s)/g, '\n');
    // 同一列表内连续项之间仅保留单换行
    s = s.replace(/(\n[-*+]\s[^\n]+)\n{2,}(?=[-*+]\s)/g, '$1\n');
    s = s.replace(/(\n\d+\.\s[^\n]+)\n{2,}(?=\d+\.\s)/g, '$1\n');
    return s;
}

function formatMarkdown(text) {
    if (!text) return '';
    text = normalizeMarkdownText(text);
    if (typeof marked !== 'undefined') {
        try {
            return marked.parse(text, { breaks: false, gfm: true });
        } catch (e) {
            // fallback
        }
    }
    return text
        .replace(/&/g, '&amp;')
        .replace(/</g, '&lt;')
        .replace(/>/g, '&gt;')
        .replace(/\n/g, '<br>');
}

function formatChatMarkdown(text) {
    if (!text) return '';
    const html = formatMarkdown(normalizeChatMarkdown(text));
    return html.replace(/<p>\s*<\/p>/gi, '');
}

window.formatChatMarkdown = formatChatMarkdown;
function initFileEditor() {
    const editorState = {
        currentPath: '',
        originalContent: '',
        modified: false,
        readOnly: false,
    };

    // 标签页切换
    $$('.editor-tab').forEach(tab => {
        tab.addEventListener('click', () => {
            const mode = tab.dataset.tab;
            switchEditorMode(mode);
        });
    });

    // 关闭编辑器
    $('#close-file-editor-modal').addEventListener('click', () => {
        if (editorState.modified && !editorState.readOnly) {
            if (!confirm('有未保存的更改，确定要关闭吗？')) {
                return;
            }
        }
        hideModal('file-editor-modal');
    });

    $('#cancel-file-editor').addEventListener('click', () => {
        if (editorState.modified && !editorState.readOnly) {
            if (!confirm('有未保存的更改，确定要取消吗？')) {
                return;
            }
        }
        hideModal('file-editor-modal');
    });

    // 保存文件
    $('#save-file-editor').addEventListener('click', async () => {
        if (editorState.readOnly) {
            showNotification('此文件为只读模式', 'error');
            return;
        }
        
        const content = $('#file-editor-textarea').value;
        try {
            await api.saveFile(editorState.currentPath, content);
            editorState.originalContent = content;
            editorState.modified = false;
            updateEditorStatus('saved');
            
            // 刷新数据
            await loadRequirementsData();
            
            // 显示成功提示
            showNotification('保存成功', 'success');
            
            // 关闭文件编辑器窗口
            setTimeout(() => {
                hideModal('file-editor-modal');
            }, 500);
        } catch (error) {
            showNotification('保存失败: ' + error.message, 'error');
        }
    });

    // 监听内容变化
    $('#file-editor-textarea').addEventListener('input', () => {
        if (editorState.readOnly) return;
        
        const content = $('#file-editor-textarea').value;
        editorState.modified = content !== editorState.originalContent;
        updateEditorStatus(editorState.modified ? 'modified' : 'saved');
        
        // 实时更新预览
        updatePreview(content);
    });

    // 暴露打开编辑器的函数
    window.openFileEditor = async (path, content = null, readOnly = false) => {
        editorState.currentPath = path;
        editorState.readOnly = readOnly;
        
        // 如果没有提供内容，从服务器加载
        if (content === null) {
            try {
                const response = await api.readFile(path);
                content = response.content;
            } catch (error) {
                showNotification('加载文件失败: ' + error.message, 'error');
                return;
            }
        }
        
        editorState.originalContent = content;
        editorState.modified = false;
        
        const textarea = $('#file-editor-textarea');
        textarea.value = content;
        textarea.readOnly = readOnly;
        
        $('#editor-file-path').textContent = path + (readOnly ? ' (只读)' : '');
        $('#file-editor-title').textContent = readOnly ? '查看文件' : '文件编辑';
        
        // 只读模式下隐藏保存按钮
        const saveBtn = $('#save-file-editor');
        if (readOnly) {
            saveBtn.style.display = 'none';
            updateEditorStatus('readonly');
        } else {
            saveBtn.style.display = 'inline-block';
            updateEditorStatus('saved');
        }
        
        updatePreview(content);
        
        // 默认显示编辑模式
        switchEditorMode('edit');
        
        showModal('file-editor-modal');
    };

    function switchEditorMode(mode) {
        $$('.editor-tab').forEach(tab => {
            tab.classList.toggle('active', tab.dataset.tab === mode);
        });

        const container = $('.editor-container');
        const editPane = $('.edit-pane');
        const previewPane = $('.preview-pane');

        if (mode === 'edit') {
            container.classList.remove('split-mode');
            editPane.classList.add('active');
            previewPane.classList.remove('active');
        } else if (mode === 'preview') {
            container.classList.remove('split-mode');
            editPane.classList.remove('active');
            previewPane.classList.add('active');
        } else if (mode === 'split') {
            container.classList.add('split-mode');
            editPane.classList.add('active');
            previewPane.classList.add('active');
        }
    }

    function updatePreview(content) {
        $('#file-editor-preview').innerHTML = formatMarkdown(content);
    }

    function updateEditorStatus(status) {
        const statusEl = $('#editor-status');
        if (status === 'saved') {
            statusEl.textContent = '已保存';
            statusEl.className = 'editor-status saved';
        } else if (status === 'modified') {
            statusEl.textContent = '未保存';
            statusEl.className = 'editor-status modified';
        } else if (status === 'readonly') {
            statusEl.textContent = '只读';
            statusEl.className = 'editor-status readonly';
        }
    }
}

function showNotification(message, type = 'info') {
    // 简单的通知实现
    const notification = document.createElement('div');
    notification.className = `notification notification-${type}`;
    notification.textContent = message;
    notification.style.cssText = `
        position: fixed;
        top: 80px;
        right: 20px;
        padding: 12px 20px;
        background: ${type === 'success' ? '#10b981' : type === 'error' ? '#ef4444' : '#3b82f6'};
        color: white;
        border-radius: 8px;
        box-shadow: 0 4px 6px rgba(0, 0, 0, 0.1);
        z-index: 10000;
        animation: slideIn 0.3s ease-out;
    `;
    
    document.body.appendChild(notification);
    
    setTimeout(() => {
        notification.style.animation = 'slideOut 0.3s ease-out';
        setTimeout(() => notification.remove(), 300);
    }, 3000);
}

// 初始化
document.addEventListener('DOMContentLoaded', () => {
    initNavigation();
    initExecution();
    initModals();
    ChatPage.init();

    // 事件绑定立即执行，不等 Bridge
    WorkspaceSelector.bindEvents();

    // Bridge 调用等 Wails runtime 就绪
    function waitForWails(cb) {
        if (window.go && window.go.desktop && window.go.desktop.Bridge) {
            cb();
        } else {
            setTimeout(() => waitForWails(cb), 50);
        }
    }
    waitForWails(() => WorkspaceSelector.loadData());
});

// ==================== 配置页面功能 ====================

class ConfigPageController {
    constructor() {
        this.settings = null;
        this.mcpServers = {};
        this.mcpServerStatuses = {};
        this.mcpConnecting = false;
        this.jsonValid = true;
        
        $('#save-config-btn')?.addEventListener('click', () => this.saveConfig());
        $('#format-mcp-json-btn')?.addEventListener('click', () => this.formatMCPJson());
        $('#validate-mcp-json-btn')?.addEventListener('click', () => this.validateMCPJson());
        
        // JSON 编辑器变化监听
        const editor = $('#mcp-json-editor');
        if (editor) {
            editor.addEventListener('input', () => this.onMCPJsonChange());
        }

        $('#mcp-servers-list')?.addEventListener('click', (e) => {
            if (e.target.closest('.mcp-toggle-switch') || e.target.closest('button')) return;
            const header = e.target.closest('.mcp-server-header');
            if (!header) return;
            const card = header.closest('.mcp-server-card');
            if (!card) return;
            card.classList.toggle('expanded');
            header.setAttribute('aria-expanded', card.classList.contains('expanded') ? 'true' : 'false');
        });
    }

    async loadConfig() {
        const loadingEl = $('#config-loading');
        const errorEl = $('#config-error');
        const container = $('#config-editor-container');

        loadingEl.style.display = 'block';
        errorEl.style.display = 'none';
        container.style.display = 'none';

        try {
            this.settings = await WailsAPI.getSettings();
            // 转为普通对象，避免 Wails 模型实例影响 JSON.stringify
            this.mcpServers = JSON.parse(JSON.stringify(this.settings.mcpServers || {}));
            
            this.render();
            loadingEl.style.display = 'none';
            container.style.display = 'block';

            // 自动连接 MCP 服务器
            await this.connectAllMCPServers({ silent: true });
        } catch (e) {
            loadingEl.style.display = 'none';
            errorEl.textContent = `加载配置失败: ${e.message}`;
            errorEl.style.display = 'block';
        }
    }

    render() {
        const s = this.settings;
        $('#model-base-url').value = s.model?.baseUrl || '';
        $('#model-api-key').value  = s.model?.apiKey  || '';
        $('#model-name').value     = s.model?.model   || '';
        $('#model-max-context-tokens').value = s.model?.maxContextTokens ?? 130000;
        $('#model-smart-compress-threshold').value = s.model?.smartCompressThreshold ?? 100000;
        
        this.renderMCPJsonEditor();
        this.renderMCPServersPreview();
    }

    renderMCPJsonEditor() {
        const editor = $('#mcp-json-editor');
        if (!editor) return;
        
        const jsonData = {
            mcpServers: this.mcpServers || {}
        };
        
        editor.value = JSON.stringify(jsonData, null, 2);
        this.jsonValid = true;
        $('#mcp-json-error').style.display = 'none';
    }

    onMCPJsonChange() {
        const editor = $('#mcp-json-editor');
        const errorEl = $('#mcp-json-error');
        
        try {
            const jsonText = editor.value.trim();
            if (!jsonText) {
                this.mcpServers = {};
                this.jsonValid = true;
                errorEl.style.display = 'none';
                this.renderMCPServersPreview();
                return;
            }
            
            const parsed = JSON.parse(jsonText);
            
            // 验证结构
            if (!parsed.mcpServers || typeof parsed.mcpServers !== 'object') {
                throw new Error('JSON 必须包含 "mcpServers" 对象');
            }
            
            this.mcpServers = parsed.mcpServers;
            this.jsonValid = true;
            errorEl.style.display = 'none';
            this.renderMCPServersPreview();
        } catch (e) {
            this.jsonValid = false;
            errorEl.textContent = `JSON 解析错误: ${e.message}`;
            errorEl.style.display = 'block';
        }
    }

    formatMCPJson() {
        const editor = $('#mcp-json-editor');
        try {
            const jsonText = editor.value.trim();
            if (!jsonText) return;
            
            const parsed = JSON.parse(jsonText);
            editor.value = JSON.stringify(parsed, null, 2);
            this.onMCPJsonChange();
            showNotification('JSON 格式化成功', 'success');
        } catch (e) {
            showNotification(`格式化失败: ${e.message}`, 'error');
        }
    }

    validateMCPJson() {
        const editor = $('#mcp-json-editor');
        const errorEl = $('#mcp-json-error');
        
        try {
            const jsonText = editor.value.trim();
            if (!jsonText) {
                showNotification('JSON 为空', 'info');
                return;
            }
            
            const parsed = JSON.parse(jsonText);
            
            if (!parsed.mcpServers || typeof parsed.mcpServers !== 'object') {
                throw new Error('JSON 必须包含 "mcpServers" 对象');
            }
            
            // 验证每个服务器配置
            const servers = parsed.mcpServers;
            for (const [name, config] of Object.entries(servers)) {
                if (typeof config !== 'object') {
                    throw new Error(`服务器 "${name}" 的配置必须是对象`);
                }
                
                const hasCommand = !!config.command;
                const hasUrl = !!config.url;
                
                if (!hasCommand && !hasUrl) {
                    throw new Error(`服务器 "${name}" 必须包含 command 或 url 字段`);
                }
                
                if (hasCommand && hasUrl) {
                    throw new Error(`服务器 "${name}" 不能同时包含 command 和 url 字段`);
                }
            }
            
            this.jsonValid = true;
            errorEl.style.display = 'none';
            showNotification('JSON 验证通过', 'success');
        } catch (e) {
            this.jsonValid = false;
            errorEl.textContent = `验证失败: ${e.message}`;
            errorEl.style.display = 'block';
            showNotification(`验证失败: ${e.message}`, 'error');
        }
    }

    renderMCPServersPreview() {
        const container = $('#mcp-servers-list');
        const countEl = $('#mcp-server-count');
        const servers = this.mcpServers || {};
        const serverNames = Object.keys(servers);

        countEl.textContent = `${serverNames.length} 个服务器`;

        if (serverNames.length === 0) {
            container.innerHTML = '<div class="empty-state">暂无 MCP 服务器配置</div>';
            return;
        }

        container.innerHTML = serverNames.map(name => {
            const server = servers[name];
            const isLocal = !!server.command;
            const isDisabled = server.disabled;
            
            // 获取服务器状态
            const status = this.mcpServerStatuses?.[name];
            
            // 获取首字母
            const initial = name.charAt(0).toUpperCase();
            
            // 生成描述文本
            let description = '';
            if (status && status.available) {
                description = `${status.toolCount} tools enabled`;
                if (isLocal && server.args && server.args.length > 0) {
                    // 如果有 resources，添加到描述中
                    const resourceCount = 4; // 这里可以从实际数据获取
                    description = `${status.toolCount} tools, ${resourceCount} resources enabled`;
                }
            } else if (isDisabled) {
                description = '已禁用';
            } else if (this.mcpConnecting) {
                description = '正在连接...';
            } else if (status && status.error) {
                description = '连接失败';
            } else {
                description = '等待连接';
            }
            
            return `
                <div class="mcp-server-card expanded ${isDisabled ? 'disabled' : ''}" data-server="${name}">
                    <div class="mcp-server-header">
                        <div class="mcp-server-name-row">
                            <div class="mcp-server-icon">${initial}</div>
                            <div class="mcp-server-info-column">
                                <div class="mcp-server-name">${name}</div>
                                <div class="mcp-server-description">${description}</div>
                            </div>
                        </div>
                        <div class="mcp-server-actions">
                            <button class="btn btn-sm btn-secondary" onclick="configPageController.testMCPServer('${name}')" ${isDisabled ? 'disabled' : ''} title="测试服务器" style="display: none;">
                                🔍
                            </button>
                            <div class="mcp-toggle-switch ${isDisabled ? '' : 'active'}" onclick="configPageController.toggleMCPServer('${name}')" title="${isDisabled ? '启用' : '禁用'}服务器"></div>
                        </div>
                    </div>
                    <div class="mcp-server-body">
                        <div class="mcp-server-info">
                            ${isLocal ? `
                                <div class="mcp-server-info-row">
                                    <span class="mcp-server-info-label">命令:</span>
                                    <span class="mcp-server-info-value">${server.command || '-'}</span>
                                </div>
                                ${server.args && server.args.length > 0 ? `
                                    <div class="mcp-server-info-row">
                                        <span class="mcp-server-info-label">参数:</span>
                                        <span class="mcp-server-info-value">${server.args.join(' ')}</span>
                                    </div>
                                ` : ''}
                                ${server.env && Object.keys(server.env).length > 0 ? `
                                    <div class="mcp-server-info-row">
                                        <span class="mcp-server-info-label">环境变量:</span>
                                        <span class="mcp-server-info-value">${Object.keys(server.env).length} 个</span>
                                    </div>
                                ` : ''}
                            ` : `
                                <div class="mcp-server-info-row">
                                    <span class="mcp-server-info-label">URL:</span>
                                    <span class="mcp-server-info-value">${server.url || '-'}</span>
                                </div>
                            `}
                            ${server.timeout ? `
                                <div class="mcp-server-info-row">
                                    <span class="mcp-server-info-label">超时:</span>
                                    <span class="mcp-server-info-value">${server.timeout}s</span>
                                </div>
                            ` : ''}
                        </div>
                        ${status && status.available && status.tools && status.tools.length > 0 ? `
                            <div class="mcp-tools-list">
                                <div class="mcp-tools-list-title">可用工具 (${status.toolCount}):</div>
                                ${status.tools.slice(0, 10).map(tool => `<div class="mcp-tool-item">${tool}</div>`).join('')}
                                ${status.tools.length > 10 ? `<div class="mcp-tool-item">... 还有 ${status.tools.length - 10} 个工具</div>` : ''}
                            </div>
                        ` : ''}
                        ${server.autoApprove && server.autoApprove.length > 0 ? `
                            <div class="mcp-tools-list">
                                <div class="mcp-tools-list-title">自动批准工具 (${server.autoApprove.length}):</div>
                                ${server.autoApprove.map(tool => `<div class="mcp-tool-item">${tool}</div>`).join('')}
                            </div>
                        ` : ''}
                        ${status && status.error ? `
                            <div class="mcp-error-message">${status.error}</div>
                        ` : ''}
                        ${status && status.lastTested ? `
                            <div class="mcp-last-tested">最后测试: ${status.lastTested}</div>
                        ` : ''}
                    </div>
                </div>
            `;
        }).join('');
    }

    toggleMCPServer(name) {
        if (!this.mcpServers[name]) return;
        
        // 切换禁用状态
        this.mcpServers[name].disabled = !this.mcpServers[name].disabled;
        
        // 更新 JSON 编辑器
        this.renderMCPJsonEditor();
        
        // 重新渲染预览
        this.renderMCPServersPreview();
        
        showNotification(
            `服务器 "${name}" 已${this.mcpServers[name].disabled ? '禁用' : '启用'}`,
            'success'
        );
    }

    async testMCPServer(name) {
        try {
            showNotification(`正在测试 MCP 服务器 "${name}"...`, 'info');
            
            const status = await WailsAPI.testMCPServer(name);
            this.mcpServerStatuses[name] = status;
            this.renderMCPServersPreview();
            
            if (status.available) {
                showNotification(`服务器 "${name}" 可用，发现 ${status.toolCount} 个工具`, 'success');
            } else {
                showNotification(`服务器 "${name}" 不可用: ${status.error || '未知错误'}`, 'error');
            }
        } catch (error) {
            showNotification(`测试失败: ${error.message}`, 'error');
        }
    }

    async connectAllMCPServers({ silent = false } = {}) {
        const servers = this.mcpServers || {};
        if (Object.keys(servers).length === 0) {
            this.mcpServerStatuses = {};
            this.renderMCPServersPreview();
            return;
        }

        this.mcpConnecting = true;
        this.renderMCPServersPreview();

        try {
            const timeoutMs = 90000;
            const statuses = await Promise.race([
                WailsAPI.connectAllMCPServers(),
                new Promise((_, reject) =>
                    setTimeout(() => reject(new Error('连接超时（90秒），请检查 Node.js / npx 或网络')), timeoutMs)
                ),
            ]);
            this.mcpServerStatuses = statuses || {};
            if (!silent) {
                const available = Object.values(statuses).filter(s => s.available).length;
                const total = Object.keys(statuses).length;
                showNotification(`MCP 已连接: ${available}/${total} 个服务器可用`, 'success');
            }
        } catch (error) {
            console.error('MCP auto-connect failed:', error);
            if (!silent) {
                showNotification(`MCP 连接失败: ${error.message}`, 'error');
            }
        } finally {
            this.mcpConnecting = false;
            this.renderMCPServersPreview();
        }
    }

    collectSettings() {
        const s = {
            model: {
                baseUrl: $('#model-base-url').value.trim(),
                apiKey:  $('#model-api-key').value.trim(),
                model:   $('#model-name').value.trim(),
                maxContextTokens: parseInt($('#model-max-context-tokens').value, 10) || 130000,
                smartCompressThreshold: parseInt($('#model-smart-compress-threshold').value, 10) || 100000,
            },
            mcpServers: this.mcpServers,
        };
        
        return s;
    }

    async saveConfig() {
        const successEl = $('#config-success');
        const errorEl   = $('#config-error');
        const btn       = $('#save-config-btn');

        // 验证 JSON
        if (!this.jsonValid) {
            showNotification('请先修复 JSON 错误', 'error');
            return;
        }

        successEl.style.display = 'none';
        errorEl.style.display   = 'none';
        btn.disabled = true;
        btn.innerHTML = '<span class="btn-icon">⏳</span> 保存中...';

        try {
            const s = this.collectSettings();
            await WailsAPI.saveSettings(s);
            this.settings = s;
            await this.connectAllMCPServers({ silent: true });
            successEl.textContent = '配置已保存';
            successEl.style.display = 'block';
            setTimeout(() => { successEl.style.display = 'none'; }, 3000);
        } catch (e) {
            errorEl.textContent = `保存失败: ${e.message}`;
            errorEl.style.display = 'block';
        } finally {
            btn.disabled = false;
            btn.innerHTML = '<span class="btn-icon">💾</span> 保存配置';
        }
    }
}

let configPageController = null;

const originalLoadPageData = loadPageData;
loadPageData = async function(pageName) {
    if (pageName === 'config') {
        if (!configPageController) {
            configPageController = new ConfigPageController();
        }
        await configPageController.loadConfig();
    } else {
        await originalLoadPageData(pageName);
    }
};


// ==================== 自由对话页面 ====================

const ChatPage = (() => {
    const STORAGE_KEY_PREFIX = 'chat_sessions';

    let sessions = [];
    let activeId = null;
    let workspacePath = '';
    let workspaceId = '';
    /** 已从存储加载过的工作区 key；未 hydrate 前禁止 save，避免启动时用空列表覆盖 chat-history.json */
    let hydratedKey = null;
    let isRunning = false;
    let chatSessionView = null;
    let chatSubAgentPanel = null;

    function normalizeWorkspacePath(p) {
        if (!p || typeof p !== 'string') return '';
        return p.trim().replace(/\\/g, '/');
    }

    function storageKey() {
        if (workspaceId) return `${STORAGE_KEY_PREFIX}:${workspaceId}`;
        const ws = normalizeWorkspacePath(workspacePath || WorkspaceSelector.currentPath || '');
        if (!ws) return `${STORAGE_KEY_PREFIX}:default`;
        return `${STORAGE_KEY_PREFIX}:${ws}`;
    }

    function getChatSubAgentPanel() {
        const root = document.querySelector('#chat-subagent-panel-root');
        if (!root || !window.SubAgentPanel) return null;
        if (!chatSubAgentPanel) {
            chatSubAgentPanel = createSubAgentPanel(root, getChatSessionView);
        }
        return chatSubAgentPanel;
    }

    function getChatSessionView() {
        const panel = document.querySelector('#chat-session-panel');
        if (!panel || !window.SessionView) return null;
        if (!chatSessionView) {
            // 与执行任务页相同组件与选项；仅外层布局（live strip）不同
            chatSessionView = window.SessionView.create(panel, {
                subagentPanel: getChatSubAgentPanel(),
            });
            bindChatSessionPanelResize(panel);
        }
        return chatSessionView;
    }

    const CHAT_PANEL_HEIGHT_KEY = 'matrix:chat-session-panel-height';

    function restoreChatSessionPanelHeight(panel) {
        if (!panel) return;
        try {
            const saved = localStorage.getItem(CHAT_PANEL_HEIGHT_KEY);
            if (saved && /^\d+$/.test(saved)) {
                panel.style.setProperty('--chat-session-panel-height', `${saved}px`);
            }
        } catch { /* ignore */ }
    }

    function bindChatSessionPanelResize(panel) {
        if (!panel || panel.dataset.resizeBound) return;
        panel.dataset.resizeBound = '1';
        restoreChatSessionPanelHeight(panel);
        let saveTimer = null;
        const ro = new ResizeObserver((entries) => {
            const h = Math.round(entries[0]?.contentRect?.height || 0);
            if (h < 100) return;
            clearTimeout(saveTimer);
            saveTimer = setTimeout(() => {
                try {
                    localStorage.setItem(CHAT_PANEL_HEIGHT_KEY, String(h));
                    panel.style.setProperty('--chat-session-panel-height', `${h}px`);
                } catch { /* ignore */ }
            }, 200);
        });
        ro.observe(panel);
    }

    function syncChatLiveStrip() {
        const strip = document.querySelector('#chat-live-strip');
        const sessionPanel = document.querySelector('#chat-session-panel');
        const subRoot = document.querySelector('#chat-subagent-panel-root');
        if (!strip) return;
        const sessionVisible = sessionPanel && sessionPanel.style.display !== 'none';
        const hasAgents = subRoot?.classList.contains('has-agents');
        strip.hidden = !sessionVisible && !hasAgents;
        strip.classList.toggle('has-session-panel', !!sessionVisible);
        if (sessionVisible) bindChatSessionPanelResize(sessionPanel);
        if (!strip.hidden) {
            scrollChatToBottom();
        }
    }

    function scrollChatToBottom() {
        const scrollArea = document.querySelector('.chat-scroll-area');
        if (!scrollArea) return;
        requestAnimationFrame(() => {
            scrollArea.scrollTop = scrollArea.scrollHeight;
        });
    }

    window.syncChatLiveStrip = syncChatLiveStrip;

    function snapshotStepLabel(snap) {
        if (!snap) return '0 步';
        if (snap.stats && window.SessionView?.formatStepCountLabel) {
            return window.SessionView.formatStepCountLabel(snap.stats);
        }
        const feed = snap.feed || snap.timeline || [];
        const toolCount = snap.tools ? Object.keys(snap.tools).length : 0;
        let workerTools = 0;
        if (snap.workers) {
            Object.values(snap.workers).forEach((w) => {
                workerTools += Object.keys(w.state?.tools || {}).length;
            });
        }
        const turns = snap.turn || 0;
        const parts = [];
        if (turns > 0) parts.push(`${turns} 轮`);
        const tools = toolCount + workerTools;
        if (tools > 0) parts.push(`${tools} 次工具`);
        if (snap.workers && Object.keys(snap.workers).length > 0) {
            parts.push(`${Object.keys(snap.workers).length} 个 Worker`);
        }
        return parts.length ? parts.join(' · ') : `${Math.max(feed.length, toolCount)} 步`;
    }

    function snapshotStepCount(snap) {
        if (!snap) return 0;
        if (snap.stats) {
            return (snap.stats.toolCalls || 0) + (snap.stats.turns || 0);
        }
        const feed = snap.feed || snap.timeline || [];
        const toolCount = snap.tools ? Object.keys(snap.tools).length : 0;
        return Math.max(feed.length, toolCount);
    }

    function hasMeaningfulSnapshot(snap) {
        if (!snap) return false;
        if (snap.thinkingText?.trim()) return true;
        if ((snap.feed || snap.timeline || []).length > 0) return true;
        if (snap.tools && Object.keys(snap.tools).length > 0) return true;
        if (snap.workers && Object.keys(snap.workers).length > 0) return true;
        if (snap.todos?.length > 0) return true;
        return false;
    }

    async function load() {
        sessions = [];
        try {
            if (window.WailsAPI?.getChatSessions) {
                const fromBackend = await window.WailsAPI.getChatSessions();
                sessions = Array.isArray(fromBackend) ? fromBackend : [];
                return;
            }
        } catch (e) {
            console.warn('[ChatPage] load from backend failed:', e);
        }
        const key = storageKey();
        let raw = null;
        try { raw = localStorage.getItem(key); } catch { raw = null; }
        try { sessions = JSON.parse(raw || '[]'); } catch { sessions = []; }
    }

    async function save() {
        if (!hydratedKey) return;
        try {
            if (window.WailsAPI?.saveChatSessions) {
                await window.WailsAPI.saveChatSessions(sessions);
                return;
            }
        } catch (e) {
            console.warn('[ChatPage] save to backend failed:', e);
        }
        try { localStorage.setItem(storageKey(), JSON.stringify(sessions)); } catch {}
    }

    /** 切换工作区前持久化当前内存中的会话（须在 SetWorkspace 之前调用） */
    async function persistIfLoaded() {
        const key = storageKey();
        if (!hydratedKey || hydratedKey !== key) return;
        await save();
    }

    function markHydrated() {
        hydratedKey = storageKey();
    }

    async function onWorkspaceChanged(path, id) {
        workspacePath = normalizeWorkspacePath(path || WorkspaceSelector.currentPath || '');
        workspaceId = id || WorkspaceSelector.workspaceId || '';
        activeId = null;
        hydratedKey = null;
        await load();
        markHydrated();
        if (sessions.length === 0) {
            newSession();
        } else {
            activeId = sessions[0].id;
            renderSidebar();
            renderMessages();
        }
    }

    function activeSession() {
        return sessions.find(s => s.id === activeId) || null;
    }

    function newSession() {
        const id = Date.now().toString();
        sessions.unshift({ id, title: '新会话', messages: [] });
        activeId = id;
        save();
        renderSidebar();
        renderMessages();
    }

    function selectSession(id) {
        activeId = id;
        renderSidebar();
        renderMessages();
    }

    function deleteSession(id) {
        sessions = sessions.filter(s => s.id !== id);
        if (activeId === id) activeId = sessions.length > 0 ? sessions[0].id : null;
        save();
        if (window.WailsAPI.clearChatSession) {
            window.WailsAPI.clearChatSession(id).catch(() => {});
        }
        renderSidebar();
        renderMessages();
    }

    function bootstrapTurns(session) {
        if (!session || !session.messages) return [];
        return session.messages.map(m => ({
            role: m.role,
            content: m.content || '',
        }));
    }

    function renderSidebar() {
        const list = document.querySelector('#chat-session-list');
        if (!list) return;
        if (sessions.length === 0) {
            list.innerHTML = '<div class="chat-session-empty">暂无会话</div>';
            return;
        }
        list.innerHTML = sessions.map(s => `
            <div class="chat-session-item ${s.id === activeId ? 'active' : ''}" data-id="${s.id}">
                <span class="chat-session-title">${escapeHtml(s.title)}</span>
                <button class="chat-session-del" data-id="${s.id}" title="删除">✕</button>
            </div>
        `).join('');
        list.querySelectorAll('.chat-session-item').forEach(el => {
            el.addEventListener('click', (e) => {
                if (e.target.classList.contains('chat-session-del')) return;
                selectSession(el.dataset.id);
            });
        });
        list.querySelectorAll('.chat-session-del').forEach(btn => {
            btn.addEventListener('click', (e) => {
                e.stopPropagation();
                if (confirm('删除该会话？')) deleteSession(btn.dataset.id);
            });
        });
    }

    function renderMessages() {
        const container = document.querySelector('#chat-messages');
        const empty = document.querySelector('#chat-empty');
        if (!container) return;

        // 清除旧消息（保留 chat-empty 节点）
        Array.from(container.children).forEach(c => {
            if (c.id !== 'chat-empty') c.remove();
        });

        const session = activeSession();
        const msgs = session ? session.messages : [];

        if (msgs.length === 0) {
            if (empty) empty.style.display = 'flex';
            return;
        }
        if (empty) empty.style.display = 'none';

        msgs.forEach(msg => container.appendChild(buildMessageEl(msg)));
        scrollChatToBottom();
    }

    function buildMessageEl(msg) {
        const div = document.createElement('div');
        div.className = `chat-msg chat-msg-${msg.role}`;
        const bubble = document.createElement('div');
        bubble.className = 'chat-bubble';

        if (msg.role === 'user') {
            bubble.textContent = msg.content;
        } else {
            const mdDiv = document.createElement('div');
            mdDiv.className = 'chat-output markdown-content';
            mdDiv.innerHTML = formatChatMarkdown(msg.content || '（无输出）');
            bubble.appendChild(mdDiv);
            if (hasMeaningfulSnapshot(msg.sessionSnapshot)) {
                const details = document.createElement('details');
                details.className = 'chat-logs-details';
                const summary = document.createElement('summary');
                const label = snapshotStepLabel(msg.sessionSnapshot);
                summary.textContent = `查看执行过程（${label}）`;
                details.appendChild(summary);
                const inner = document.createElement('div');
                inner.className = 'chat-logs-inner session-view session-view-compact session-view-in-bubble';
                // 气泡内复盘用 compact；实时执行区与执行任务页一致，不用 compact
                window.SessionView.create(inner, { compact: true }).loadSnapshot(msg.sessionSnapshot);
                details.appendChild(inner);
                bubble.appendChild(details);
            }
        }

        const meta = document.createElement('div');
        meta.className = 'chat-meta';
        meta.textContent = msg.time || '';
        div.appendChild(bubble);
        div.appendChild(meta);
        return div;
    }

    function nowTime() {
        const n = new Date();
        return `${String(n.getHours()).padStart(2,'0')}:${String(n.getMinutes()).padStart(2,'0')}:${String(n.getSeconds()).padStart(2,'0')}`;
    }

    function escapeHtml(s) {
        return String(s).replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;');
    }

    async function send() {
        if (isRunning) return;
        const input = document.querySelector('#chat-input');
        const text = input ? input.value.trim() : '';
        if (!text) return;

        console.log('[ChatPage] Sending message:', text);

        if (!activeId) newSession();
        const session = activeSession();
        if (session.messages.length === 0) {
            session.title = text.slice(0, 20) + (text.length > 20 ? '…' : '');
        }

        session.messages.push({ role: 'user', content: text, time: nowTime() });
        save();
        if (input) input.value = '';
        renderSidebar();
        renderMessages();

        isRunning = true;
        setSendDisabled(true);

        const panelEl = document.querySelector('#chat-session-panel');
        const view = getChatSessionView();
        if (panelEl) {
            panelEl.style.display = 'block';
            syncChatLiveStrip();
        }
        if (view) view.reset();

        let snapshot = null;
        const chatSubPanel = getChatSubAgentPanel();

        try {
            if (chatSubPanel) chatSubPanel.clear();
            const bootstrap = bootstrapTurns(session);
            await window.WailsAPI.runChatSession(
                session.id,
                text,
                bootstrap.slice(0, -1),
                (msg) => {
                    if (view) view.apply(msg);
                    if (view) snapshot = view.getSnapshot();
                    scrollChatToBottom();
                },
                (result) => {
                    if (result?.has_error) {
                    if (panelEl) {
                        panelEl.style.display = 'none';
                        syncChatLiveStrip();
                    }
                    if (chatSubPanel) chatSubPanel.clear();
                    if (view) view.finalizeRunningTools('error');
                    session.messages.push({
                        role: 'assistant',
                        content: `执行失败: ${result.error || '未知错误'}`,
                            time: nowTime(),
                            sessionSnapshot: view ? view.getSnapshot() : snapshot,
                        });
                        save();
                        renderSidebar();
                        renderMessages();
                        isRunning = false;
                        setSendDisabled(false);
                        return;
                    }
                    setTimeout(() => {
                        if (panelEl) panelEl.style.display = 'none';
                        if (chatSubPanel) chatSubPanel.clear();
                        syncChatLiveStrip();
                    }, 400);
                    if (view) view.finalizeRunningTools('done');
                    const content = (view && view.getState().assistantText) || result.output || '（任务完成，无文本输出）';
                    const finalSnap = snapshot || (view ? view.getSnapshot() : null);
                    session.messages.push({
                        role: 'assistant',
                        content,
                        time: nowTime(),
                        sessionSnapshot: hasMeaningfulSnapshot(finalSnap) ? finalSnap : null,
                    });
                    save();
                    renderSidebar();
                    renderMessages();
                    isRunning = false;
                    setSendDisabled(false);
                },
                (err) => {
                    if (panelEl) {
                        panelEl.style.display = 'none';
                        syncChatLiveStrip();
                    }
                    if (chatSubPanel) chatSubPanel.clear();
                    if (view) view.finalizeRunningTools('error');
                    session.messages.push({
                        role: 'assistant',
                        content: `执行失败: ${err.error || '未知错误'}`,
                        time: nowTime(),
                        sessionSnapshot: view ? view.getSnapshot() : snapshot,
                    });
                    save();
                    renderSidebar();
                    renderMessages();
                    isRunning = false;
                    setSendDisabled(false);
                },
                subAgentStreamHooks(chatSubPanel)
            );
        } catch (e) {
            if (panelEl) {
                panelEl.style.display = 'none';
                syncChatLiveStrip();
            }
            getChatSubAgentPanel()?.clear();
            const failView = getChatSessionView();
            if (failView) failView.finalizeRunningTools('error');
            session.messages.push({
                role: 'assistant',
                content: `执行失败: ${e.message}`,
                time: nowTime(),
                sessionSnapshot: failView ? failView.getSnapshot() : snapshot,
            });
            save();
            renderSidebar();
            renderMessages();
            isRunning = false;
            setSendDisabled(false);
        }
    }

    function setSendDisabled(disabled) {
        const btn = document.querySelector('#chat-send-btn');
        const input = document.querySelector('#chat-input');
        if (btn) { btn.disabled = disabled; btn.innerHTML = disabled ? '⏳ 执行中...' : '<span class="btn-icon">▶</span> 发送'; }
        if (input) input.disabled = disabled;
    }

    function init() {
        document.querySelector('#chat-new-btn')?.addEventListener('click', () => newSession());
        document.querySelector('#chat-clear-btn')?.addEventListener('click', () => {
            if (confirm('清空所有会话历史？')) {
                const ids = sessions.map(s => s.id);
                sessions = [];
                activeId = null;
                save();
                ids.forEach(id => {
                    if (window.WailsAPI.clearChatSession) {
                        window.WailsAPI.clearChatSession(id).catch(() => {});
                    }
                });
                newSession();
            }
        });
        document.querySelector('#chat-send-btn')?.addEventListener('click', () => send());
        document.querySelector('#chat-input')?.addEventListener('keydown', (e) => {
            if (e.key === 'Enter' && !e.shiftKey) {
                e.preventDefault();
                send();
            }
        });
    }

    return {
        init,
        persistIfLoaded,
        onShow() {
            workspacePath = normalizeWorkspacePath(WorkspaceSelector.currentPath || workspacePath);
            workspaceId = WorkspaceSelector.workspaceId || workspaceId;
            syncChatLiveStrip();
            load().then(() => {
                markHydrated();
                if (sessions.length === 0) {
                    newSession();
                } else if (!activeId || !sessions.some(s => s.id === activeId)) {
                    activeId = sessions[0].id;
                }
                renderSidebar();
                renderMessages();
            });
        },
        onWorkspaceChanged,
    };
})();

// ==================== 工作区选择器 ====================

const WorkspaceSelector = {
    currentPath: '',
    workspaceId: '',

    bindEvents() {
        $('#workspace-browse-btn').style.display = '';
        $('#workspace-indicator').addEventListener('click', () => this.show());
        $('#workspace-open-btn').addEventListener('click', () => this.openSelected());
        $('#workspace-browse-btn').addEventListener('click', () => this.browseFolder());
        $('#workspace-path-input').addEventListener('keydown', (e) => {
            if (e.key === 'Enter') this.openSelected();
        });
    },

    async loadData() {
        await this.loadCurrent();
        if (!this.currentPath) {
            this.show();
        }
    },

    async loadCurrent() {
        try {
            const data = await WailsAPI.getWorkspace();
            this.currentPath = data.current || '';
            this.workspaceId = data.workspaceId || '';
            this.updateDisplay();
            this.renderRecentList(data.recent || []);
            ChatPage.onWorkspaceChanged(this.currentPath, this.workspaceId);
        } catch (e) {
            console.error('Failed to load workspace:', e);
        }
    },

    updateDisplay() {
        const el = $('#workspace-path-display');
        if (!el) return;
        if (this.currentPath) {
            const parts = this.currentPath.replace(/\\/g, '/').split('/');
            el.textContent = parts[parts.length - 1] || this.currentPath;
            el.title = this.currentPath;
        } else {
            el.textContent = '未选择工作区';
        }
    },

    renderRecentList(recent) {
        const container = $('#recent-workspaces-list');
        if (!container) return;
        const esc = (s) => String(s).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/"/g, '&quot;');
        const paths = (recent || [])
            .map(entry => (typeof entry === 'string' ? entry : entry?.path))
            .filter(Boolean);
        if (paths.length === 0) {
            container.innerHTML = '<div class="empty-state">暂无最近记录</div>';
            return;
        }
        container.innerHTML = paths.map(path => `
            <div class="recent-workspace-item" data-path="${esc(path)}">
                <span class="recent-workspace-icon">📁</span>
                <div class="recent-workspace-info">
                    <div class="recent-workspace-name">${esc(path.replace(/\\/g, '/').split('/').pop())}</div>
                    <div class="recent-workspace-path">${esc(path)}</div>
                </div>
            </div>
        `).join('');
        container.querySelectorAll('.recent-workspace-item').forEach(item => {
            item.addEventListener('click', () => this.selectPath(item.dataset.path));
        });
    },

    show() {
        $('#workspace-path-input').value = this.currentPath || '';
        $('#workspace-error').style.display = 'none';
        WailsAPI.getWorkspace().then(data => this.renderRecentList(data.recent || [])).catch(() => {});
        showModal('workspace-modal');
    },

    hide() {
        hideModal('workspace-modal');
    },

    async browseFolder() {
        try {
            const path = await WailsAPI.openFolderDialog();
            if (path) $('#workspace-path-input').value = path;
        } catch (e) {
            console.error('Failed to open folder dialog:', e);
        }
    },

    async openSelected() {
        const path = $('#workspace-path-input').value.trim();
        if (!path) return;
        await this.selectPath(path);
    },

    async selectPath(path) {
        const errEl = $('#workspace-error');
        errEl.style.display = 'none';
        try {
            await ChatPage.persistIfLoaded();
            const result = await WailsAPI.setWorkspace({ path });
            this.currentPath = result.path || path;
            this.workspaceId = result.workspaceId || '';
            this.updateDisplay();
            if (result.recent) {
                this.renderRecentList(result.recent);
            }
            ChatPage.onWorkspaceChanged(this.currentPath, this.workspaceId);
            this.hide();
            loadDashboard();
        } catch (e) {
            errEl.textContent = e.message || '无法打开该文件夹，请检查路径是否正确';
            errEl.style.display = 'block';
        }
    },
};


// 版本标记 - 用于强制刷新缓存
console.log('app.js loaded - Version: 2026-05-24-chat-hydrate-fix');
