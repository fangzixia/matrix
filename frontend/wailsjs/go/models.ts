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

}

