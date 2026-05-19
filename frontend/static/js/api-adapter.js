// API Adapter - 桌面模式，直接调用 Go Bridge

const WailsAPI = {
    getMode: () => 'desktop',

    // 工作区
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

    // 配置
    async getSettings() {
        return window.go.desktop.Bridge.GetSettings();
    },
    async saveSettings(settings) {
        await window.go.desktop.Bridge.SaveSettings(settings);
        return { success: true };
    },

    // 文件操作
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

    // 需求和评测（从文件系统读取，由 Bridge 提供）
    async getRequirements() {
        const result = await window.go.desktop.Bridge.GetRequirements();
        return { requirements: result };
    },
    async getEvaluations() {
        const result = await window.go.desktop.Bridge.GetEvaluations();
        return { evaluations: result };
    },

    // 任务执行（非流式）
    async runTask(agentName, task, filePath = '') {
        return window.go.desktop.Bridge.RunTask(agentName, task, filePath);
    },

    // 任务执行（流式）
    async runTaskStreaming(agentName, task, filePath = '', onLog, onDone, onError) {
        const eventName = 'task:progress';

        console.log('[API Adapter] Starting streaming task:', agentName, task);
        console.log('[API Adapter] window.runtime available:', !!window.runtime);
        console.log('[API Adapter] EventsOn available:', typeof window.runtime?.EventsOn);

        // 先注册事件监听器，确保不会错过任何事件
        if (onLog) {
            console.log('[API Adapter] Registering event listener for:', eventName);
            window.runtime.EventsOn(eventName, (log) => {
                console.log('[API Adapter] Received event:', log);
                onLog({ type: log.type || 'log', ...log });
            });
        }

        // 等待一小段时间确保事件监听器已注册
        await new Promise(resolve => setTimeout(resolve, 50));

        try {
            console.log('[API Adapter] Calling RunTaskWithProgress...');
            const result = await window.go.desktop.Bridge.RunTaskWithProgress(agentName, task, filePath);
            console.log('[API Adapter] Task completed:', result);
            window.runtime.EventsOff(eventName);
            if (onDone) onDone(result);
        } catch (error) {
            console.error('[API Adapter] Task error:', error);
            window.runtime.EventsOff(eventName);
            if (onError) {
                onError({ error: error.message || '任务执行失败' });
            } else {
                throw error;
            }
        }
    },

    // MCP 服务器测试
    async testMCPServer(serverName) {
        return window.go.desktop.Bridge.TestMCPServer(serverName);
    },
    async testAllMCPServers() {
        return window.go.desktop.Bridge.TestAllMCPServers();
    },
    async getMCPServerStatus(serverName) {
        return window.go.desktop.Bridge.GetMCPServerStatus(serverName);
    },
    async getAllMCPServerStatuses() {
        return window.go.desktop.Bridge.GetAllMCPServerStatuses();
    },
};

window.WailsAPI = WailsAPI;

console.log('[API Adapter] Desktop mode');
