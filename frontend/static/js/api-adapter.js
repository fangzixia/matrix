// API Adapter - 桌面模式，直接调用 Go Bridge

const STREAM_EVENT = 'agent:stream';

const WailsAPI = {
    getMode: () => 'desktop',

    async getWorkspace() {
        return window.go.desktop.Bridge.GetWorkspace();
    },
    async setWorkspace(req) {
        await window.go.desktop.Bridge.SetWorkspace(req.path || req);
        return { path: req.path || req };
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

    async runTask(agentName, task, filePath = '') {
        return window.go.desktop.Bridge.RunTask(agentName, task, filePath);
    },

    /**
     * 流式 Agent 会话：通过 agent:stream 推送 SDK 消息。
     * @param {function(object): void} onStreamMessage
     */
    async runAgentSession(agentName, task, filePath = '', onStreamMessage, onDone, onError) {
        if (onStreamMessage) {
            window.runtime.EventsOn(STREAM_EVENT, (msg) => {
                onStreamMessage(msg);
            });
        }

        await new Promise((resolve) => setTimeout(resolve, 50));

        try {
            const runFn = window.go.desktop.Bridge.RunAgentSession || window.go.desktop.Bridge.RunTaskWithProgress;
            const result = await runFn(agentName, task, filePath);
            window.runtime.EventsOff(STREAM_EVENT);
            if (onDone) onDone(result);
        } catch (error) {
            console.error('[API Adapter] Session error:', error);
            window.runtime.EventsOff(STREAM_EVENT);
            if (onError) {
                onError({ error: error.message || '任务执行失败' });
            } else {
                throw error;
            }
        }
    },

    /** @deprecated 使用 runAgentSession */
    async runTaskStreaming(agentName, task, filePath = '', onLog, onDone, onError) {
        return this.runAgentSession(agentName, task, filePath, onLog, onDone, onError);
    },

    async cancelAgentSession() {
        if (window.go.desktop.Bridge.CancelAgentSession) {
            return window.go.desktop.Bridge.CancelAgentSession();
        }
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
