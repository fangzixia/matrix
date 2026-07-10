package settings

import "time"

// defaultSettings 返回系统配置各域的 UI/合并默认值（数据库无记录时使用）。
func defaultSettings() Settings {
	return Settings{
		Models:     nil,
		Context:    defaultContextSettings(),
		Security:   defaultSecuritySettings(),
		MCPServers: map[string]MCPServerSettings{},
		Git:        defaultGitSettings(),
	}
}

func defaultContextSettings() ContextSettings {
	return ContextSettings{
		AutoCompactThreshold: 100000,
		KeepRecentMessages:   8,
	}
}

func defaultSecuritySettings() SecuritySettings {
	return SecuritySettings{ShellTimeout: (60 * time.Second).String()}
}

func defaultGitSettings() GitSettings {
	return GitSettings{CloneTimeout: (300 * time.Second).String()}
}
