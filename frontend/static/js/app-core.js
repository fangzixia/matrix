// Shared app state and small DOM utilities used by app.js.
const API_BASE = window.location.origin;

const state = {
    requirements: [],
    evaluations: [],
    currentPersona: null,
    currentRequirement: null,
    executionStartTime: null,
    /** 流式执行所属任务类型（切换卡片时 currentPersona 会变，日志须归到发起执行的任务） */
    streamingPersona: null,
    /** 每个任务卡片独立的执行过程快照 */
    executionByPersona: {},
};

const $ = (selector) => document.querySelector(selector);
const $$ = (selector) => document.querySelectorAll(selector);

const formatDate = (dateStr) => {
    if (!dateStr) return '-';
    const date = new Date(dateStr);
    return date.toLocaleString('zh-CN');
};

function showNotification(message, type = 'info') {
    console.log(`[${type.toUpperCase()}] ${message}`);

    const successEl = $('#config-success');
    const errorEl = $('#config-error');

    if (type === 'success' && successEl) {
        successEl.textContent = message;
        successEl.style.display = 'block';
        setTimeout(() => { successEl.style.display = 'none'; }, 3000);
    } else if (type === 'error' && errorEl) {
        errorEl.textContent = message;
        errorEl.style.display = 'block';
        setTimeout(() => { errorEl.style.display = 'none'; }, 5000);
    } else if (type === 'info' && successEl) {
        successEl.textContent = message;
        successEl.style.display = 'block';
        setTimeout(() => { successEl.style.display = 'none'; }, 2000);
    }
}

function createEmptyExecutionSlot() {
    return {
        sessionSnapshot: null,
        result: null,
        hasError: false,
        running: false,
    };
}
