package settings

import "time"

func defaultAISettings() AISettings {
	return AISettings{
		Context:  defaultContextSettings(),
		Security: defaultSecuritySettings(),
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
