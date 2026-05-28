// API Adapter - 桌面模式，直接调用 Go Bridge

const STREAM_EVENT = 'agent:stream';
const SUBAGENT_UPDATE = 'subagent:update';
const SUBAGENT_DONE = 'subagent:done';

function wailsErrorMessage(error, fallback) {
    if (typeof error === 'string' && error.trim()) return error.trim();
    if (error?.message) return error.message;
    try {
        const s = String(error);
        if (s && s !== '[object Object]') return s;
    } catch { /* ignore */ }
    return fallback;
}

function normalizeError(error, fallback, code = 'transport_error') {
    if (error?.error_info) return error.error_info;
    if (error?.code && error?.message) return error;
    return {
        code: error?.error_code || code,
        message: wailsErrorMessage(error, fallback),
        retryable: Boolean(error?.retryable),
    };
}

const WailsAPI = {
    getMode: () => 'desktop',

    async getWorkspace() {
        return window.go.desktop.Bridge.GetWorkspace();
    },
    async setWorkspace(req) {
        await window.go.desktop.Bridge.SetWorkspace(req.path || req);
        const ws = await window.go.desktop.Bridge.GetWorkspace();
        return {
            path: ws.current || req.path || req,
            workspaceId: ws.workspaceId || '',
            recent: ws.recent || [],
        };
    },
    async openFolderDialog() {
        return window.go.desktop.Bridge.OpenFolderDialog();
    },

    async getSettings() {
        return window.go.desktop.Bridge.GetSettings();
    },
    async saveSettings(settings) {
        await window.go.desktop.Bridge.SaveSettings(settings);
        return { success: true };
    },

    async listFiles(path) {
        const files = await window.go.desktop.Bridge.ListFiles(path);
        return { files };
    },
    async readFile(path) {
        const content = await window.go.desktop.Bridge.ReadFile(path);
        return { path, content };
    },
    async saveFile(path, content) {
        await window.go.desktop.Bridge.SaveFile(path, content);
        return { success: true, message: '保存成功' };
    },

    async getRequirements() {
        const result = await window.go.desktop.Bridge.GetRequirements();
        return { requirements: result };
    },
    async getEvaluations() {
        const result = await window.go.desktop.Bridge.GetEvaluations();
        return { evaluations: result };
    },

    async runTask(task) {
        return window.go.desktop.Bridge.RunTask(task);
    },

    /**
     * 通用 Agent 流式会话；通过 agent:stream 推送过程消息。
     * @param {function(object): void} onStreamMessage
     */
    async runAgentSession(task, onStreamMessage, onDone, onError, hooks = {}) {
        this._bindStreamHooks(onStreamMessage, hooks);
        await new Promise((resolve) => setTimeout(resolve, 50));
        try {
            const result = await window.go.desktop.Bridge.RunAgentSession(task);
            this._offStreamHooks();
            if (onDone) onDone(this._normalizeRunResult(result));
        } catch (error) {
            console.error('[API Adapter] Session error:', error);
            this._offStreamHooks();
            if (onError) {
                const info = normalizeError(error, '任务执行失败');
                onError({ error: info.message, error_info: info, error_code: info.code });
            } else {
                throw error;
            }
        }
    },

    /**
     * 自由对话多轮会话：同一 chatSessionId 在后端续接完整 Agent transcript。
     * @param {string} chatSessionId
     * @param {string} message 本轮用户输入
     * @param {Array<{role:string,content:string}>} bootstrap 无后端缓存时由前端提供的历史
     */
    async runChatSession(chatSessionId, message, bootstrap, onStreamMessage, onDone, onError, hooks = {}) {
        this._bindStreamHooks(onStreamMessage, hooks);
        await new Promise((resolve) => setTimeout(resolve, 50));
        try {
            const result = await window.go.desktop.Bridge.RunChatSession({
                chatSessionId,
                message,
                bootstrap: bootstrap || [],
            });
            this._offStreamHooks();
            if (onDone) onDone(this._normalizeRunResult(result));
        } catch (error) {
            console.error('[API Adapter] Chat session error:', error);
            this._offStreamHooks();
            if (onError) {
                const info = normalizeError(error, '对话执行失败');
                onError({ error: info.message, error_info: info, error_code: info.code });
            } else {
                throw error;
            }
        }
    },

    async getChatSessions() {
        if (window.go.desktop.Bridge.GetChatSessions) {
            return window.go.desktop.Bridge.GetChatSessions();
        }
        return [];
    },
    async saveChatSessions(sessions) {
        if (window.go.desktop.Bridge.SaveChatSessions) {
            await window.go.desktop.Bridge.SaveChatSessions(sessions || []);
        }
    },

    async clearChatSession(chatSessionId) {
        if (window.go.desktop.Bridge.ClearChatSession) {
            await window.go.desktop.Bridge.ClearChatSession(chatSessionId);
        }
    },

    _taskRunners: {
        spec: (input, file) => window.go.desktop.Bridge.RunSpec(input, file),
        implement: (input, file) => window.go.desktop.Bridge.RunImplement(input, file),
        verify: (input, file) => window.go.desktop.Bridge.RunVerify(input, file),
        build: (input, file) => window.go.desktop.Bridge.RunBuild(input, file),
        'ui-scan': (input, file) => window.go.desktop.Bridge.RunUIScan(input, file),
    },

    _bindStreamHooks(onStream, hooks = {}) {
        if (onStream) {
            window.runtime.EventsOn(STREAM_EVENT, (msg) => onStream(msg));
        }
        const { onSubAgentUpdate, onSubAgentDone } = hooks;
        if (onSubAgentUpdate) {
            window.runtime.EventsOn(SUBAGENT_UPDATE, (snap) => onSubAgentUpdate(snap));
        }
        if (onSubAgentDone) {
            window.runtime.EventsOn(SUBAGENT_DONE, (snap) => onSubAgentDone(snap));
        }
    },

    _offStreamHooks() {
        window.runtime.EventsOff(STREAM_EVENT);
        window.runtime.EventsOff(SUBAGENT_UPDATE);
        window.runtime.EventsOff(SUBAGENT_DONE);
    },

    async runTaskSession(kind, userInput, filePath = '', onStream, onDone, onError, hooks = {}) {
        const run = this._taskRunners[kind];
        this._bindStreamHooks(onStream, hooks);
        await new Promise((resolve) => setTimeout(resolve, 50));
        try {
            const result = run
                ? await run(userInput, filePath)
                : await window.go.desktop.Bridge.RunAgentSession(userInput);
            this._offStreamHooks();
            if (onDone) onDone(this._normalizeRunResult(result));
        } catch (error) {
            this._offStreamHooks();
            if (onError) {
                const info = normalizeError(error, '任务执行失败');
                onError({ error: info.message, error_info: info, error_code: info.code });
            }
            else throw error;
        }
    },

    _normalizeRunResult(result) {
        if (!result) return result;
        if (result.has_error) {
            const info = normalizeError(result, result.error || '任务执行失败', 'task_failed');
            return {
                ...result,
                error: info.message,
                error_code: info.code,
                error_info: info,
            };
        }
        return result;
    },

    /** 执行任务页：按任务类型路由到对应后端入口 */
    async runPersonaStreaming(kind, userInput, filePath = '', onStream, onDone, onError, hooks = {}) {
        return this.runTaskSession(kind, userInput, filePath, onStream, onDone, onError, hooks);
    },

    /** @deprecated 使用 runTaskSession */
    async runTaskStreaming(kind, userInput, filePath = '', onLog, onDone, onError) {
        return this.runTaskSession(kind, userInput, filePath, onLog, onDone, onError);
    },

    async cancelAgentSession() {
        if (window.go.desktop.Bridge.CancelAgentSession) {
            return window.go.desktop.Bridge.CancelAgentSession();
        }
    },

    async listSubAgents() {
        if (window.go.desktop.Bridge.ListSubAgents) {
            return window.go.desktop.Bridge.ListSubAgents();
        }
        return [];
    },

    async getSubAgent(id) {
        return window.go.desktop.Bridge.GetSubAgent(id);
    },

    async stopSubAgent(id, reason) {
        return window.go.desktop.Bridge.StopSubAgent(id, reason || '');
    },

    async readSubAgentTranscript(id, maxLines) {
        return window.go.desktop.Bridge.ReadSubAgentTranscript(id, maxLines || 80);
    },

    async testMCPServer(serverName) {
        return window.go.desktop.Bridge.TestMCPServer(serverName);
    },
    async connectAllMCPServers() {
        return window.go.desktop.Bridge.TestAllMCPServers();
    },
    async testAllMCPServers() {
        return this.connectAllMCPServers();
    },
    async getMCPServerStatus(serverName) {
        return window.go.desktop.Bridge.GetMCPServerStatus(serverName);
    },
    async getAllMCPServerStatuses() {
        return window.go.desktop.Bridge.GetAllMCPServerStatuses();
    },
};

window.WailsAPI = WailsAPI;

console.log('[API Adapter] Desktop mode (agent:stream)');
