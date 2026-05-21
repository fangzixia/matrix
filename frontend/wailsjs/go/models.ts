export namespace agent {
	
	export class Progress {
	    turn: number;
	    transition?: string;
	    summary?: string;
	    current_tool?: string;
	    tool_use_count: number;
	    last_activity?: string;
	    input_tokens?: number;
	    output_tokens?: number;
	
	    static createFrom(source: any = {}) {
	        return new Progress(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.turn = source["turn"];
	        this.transition = source["transition"];
	        this.summary = source["summary"];
	        this.current_tool = source["current_tool"];
	        this.tool_use_count = source["tool_use_count"];
	        this.last_activity = source["last_activity"];
	        this.input_tokens = source["input_tokens"];
	        this.output_tokens = source["output_tokens"];
	    }
	}

}

export namespace audit {
	
	export class Event {
	    v: number;
	    ts: string;
	    event: string;
	    session_id: string;
	    agent_id?: string;
	    parent_agent_id?: string;
	    tool_use_id?: string;
	    turn?: number;
	    component?: string;
	    level?: string;
	    data?: Record<string, any>;
	
	    static createFrom(source: any = {}) {
	        return new Event(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.v = source["v"];
	        this.ts = source["ts"];
	        this.event = source["event"];
	        this.session_id = source["session_id"];
	        this.agent_id = source["agent_id"];
	        this.parent_agent_id = source["parent_agent_id"];
	        this.tool_use_id = source["tool_use_id"];
	        this.turn = source["turn"];
	        this.component = source["component"];
	        this.level = source["level"];
	        this.data = source["data"];
	    }
	}
	export class SessionMeta {
	    session_id: string;
	    // Go type: time
	    started_at: any;
	    // Go type: time
	    ended_at?: any;
	    workspace?: string;
	    model?: string;
	    task_preview?: string;
	    stop_reason?: string;
	    turn_count?: number;
	    duration_ms?: number;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new SessionMeta(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.session_id = source["session_id"];
	        this.started_at = this.convertValues(source["started_at"], null);
	        this.ended_at = this.convertValues(source["ended_at"], null);
	        this.workspace = source["workspace"];
	        this.model = source["model"];
	        this.task_preview = source["task_preview"];
	        this.stop_reason = source["stop_reason"];
	        this.turn_count = source["turn_count"];
	        this.duration_ms = source["duration_ms"];
	        this.error = source["error"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class DiagnosticDTO {
	    session_id: string;
	    meta: SessionMeta;
	    events_tail: Event[];
	    llm_markdown: string;
	    jsonl_path: string;
	
	    static createFrom(source: any = {}) {
	        return new DiagnosticDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.session_id = source["session_id"];
	        this.meta = this.convertValues(source["meta"], SessionMeta);
	        this.events_tail = this.convertValues(source["events_tail"], Event);
	        this.llm_markdown = source["llm_markdown"];
	        this.jsonl_path = source["jsonl_path"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class SessionIndex {
	    session_id: string;
	    // Go type: time
	    started_at: any;
	    // Go type: time
	    ended_at?: any;
	    stop_reason?: string;
	    turn_count: number;
	    path: string;
	
	    static createFrom(source: any = {}) {
	        return new SessionIndex(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.session_id = source["session_id"];
	        this.started_at = this.convertValues(source["started_at"], null);
	        this.ended_at = this.convertValues(source["ended_at"], null);
	        this.stop_reason = source["stop_reason"];
	        this.turn_count = source["turn_count"];
	        this.path = source["path"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace desktop {
	
	export class MCPServerConfig {
	    command?: string;
	    args?: string[];
	    env?: Record<string, string>;
	    url?: string;
	    headers?: Record<string, string>;
	    disabled: boolean;
	    autoApprove?: string[];
	
	    static createFrom(source: any = {}) {
	        return new MCPServerConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.command = source["command"];
	        this.args = source["args"];
	        this.env = source["env"];
	        this.url = source["url"];
	        this.headers = source["headers"];
	        this.disabled = source["disabled"];
	        this.autoApprove = source["autoApprove"];
	    }
	}
	export class MCPServerStatus {
	    name: string;
	    available: boolean;
	    toolCount: number;
	    tools?: string[];
	    error?: string;
	    lastTested?: string;
	
	    static createFrom(source: any = {}) {
	        return new MCPServerStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.available = source["available"];
	        this.toolCount = source["toolCount"];
	        this.tools = source["tools"];
	        this.error = source["error"];
	        this.lastTested = source["lastTested"];
	    }
	}
	export class ModelSettings {
	    baseUrl: string;
	    apiKey: string;
	    model: string;
	    maxContextTokens: number;
	    smartCompressThreshold: number;
	
	    static createFrom(source: any = {}) {
	        return new ModelSettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.baseUrl = source["baseUrl"];
	        this.apiKey = source["apiKey"];
	        this.model = source["model"];
	        this.maxContextTokens = source["maxContextTokens"];
	        this.smartCompressThreshold = source["smartCompressThreshold"];
	    }
	}
	export class RunResult {
	    output: string;
	    has_error: boolean;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new RunResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.output = source["output"];
	        this.has_error = source["has_error"];
	        this.error = source["error"];
	    }
	}
	export class Settings {
	    model: ModelSettings;
	    mcpServers?: Record<string, MCPServerConfig>;
	
	    static createFrom(source: any = {}) {
	        return new Settings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.model = this.convertValues(source["model"], ModelSettings);
	        this.mcpServers = this.convertValues(source["mcpServers"], MCPServerConfig, true);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class SubAgentSnapshot {
	    id: string;
	    description: string;
	    status: string;
	    parent_agent_id?: string;
	    parent_tool_use_id?: string;
	    progress: agent.Progress;
	    created_at: number;
	    sidechain_path?: string;
	    answer_preview?: string;
	    turn_count?: number;
	
	    static createFrom(source: any = {}) {
	        return new SubAgentSnapshot(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.description = source["description"];
	        this.status = source["status"];
	        this.parent_agent_id = source["parent_agent_id"];
	        this.parent_tool_use_id = source["parent_tool_use_id"];
	        this.progress = this.convertValues(source["progress"], agent.Progress);
	        this.created_at = source["created_at"];
	        this.sidechain_path = source["sidechain_path"];
	        this.answer_preview = source["answer_preview"];
	        this.turn_count = source["turn_count"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

