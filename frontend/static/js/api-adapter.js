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

    async runTask(task) {
        return window.go.desktop.Bridge.RunTask(task);
    },

    /**
     * 通用 Agent 流式会话；通过 agent:stream 推送过程消息。
     * @param {function(object): void} onStreamMessage
     */
    async runAgentSession(task, onStreamMessage, onDone, onError) {
        if (onStreamMessage) {
            window.runtime.EventsOn(STREAM_EVENT, (msg) => {
                onStreamMessage(msg);
            });
        }

        await new Promise((resolve) => setTimeout(resolve, 50));

        try {
            const result = await window.go.desktop.Bridge.RunAgentSession(task);
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

    _taskRunners: {
        spec: (input, file) => window.go.desktop.Bridge.RunSpec(input, file),
        implement: (input, file) => window.go.desktop.Bridge.RunImplement(input, file),
        verify: (input, file) => window.go.desktop.Bridge.RunVerify(input, file),
        build: (input, file) => window.go.desktop.Bridge.RunBuild(input, file),
        'ui-scan': (input, file) => window.go.desktop.Bridge.RunUIScan(input, file),
    },

    async runTaskSession(kind, userInput, filePath = '', onStream, onDone, onError) {
        const run = this._taskRunners[kind];
        if (!run) {
            return this.runAgentSession(userInput, onStream, onDone, onError);
        }
        if (onStream) {
            window.runtime.EventsOn(STREAM_EVENT, (msg) => onStream(msg));
        }
        await new Promise((resolve) => setTimeout(resolve, 50));
        try {
            const result = await run(userInput, filePath);
            window.runtime.EventsOff(STREAM_EVENT);
            if (onDone) onDone(result);
        } catch (error) {
            window.runtime.EventsOff(STREAM_EVENT);
            if (onError) onError({ error: error.message || '任务执行失败' });
            else throw error;
        }
    },

    /** 执行任务页：按任务类型路由到对应后端入口 */
    async runPersonaStreaming(kind, userInput, filePath = '', onStream, onDone, onError) {
        return this.runTaskSession(kind, userInput, filePath, onStream, onDone, onError);
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
