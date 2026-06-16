package gitutil

import (
	"net/url"
	"strings"
)

// HostFromURL 从 git clone URL 解析主机名（支持 https 与 git@host:path 格式）。
func HostFromURL(gitURL string) string {
	gitURL = strings.TrimSpace(gitURL)
	if gitURL == "" {
		return ""
	}
	if strings.HasPrefix(gitURL, "git@") {
		rest := strings.TrimPrefix(gitURL, "git@")
		if i := strings.IndexAny(rest, ":/"); i >= 0 {
			return strings.ToLower(rest[:i])
		}
		return strings.ToLower(rest)
	}
	u, err := url.Parse(gitURL)
	if err != nil || u.Host == "" {
		return ""
	}
	host := u.Host
	if i := strings.Index(host, ":"); i >= 0 {
		host = host[:i]
	}
	return strings.ToLower(host)
}

// MatchHost 判断仓库主机是否匹配配置项（* 或空表示默认匹配）。
func MatchHost(repoHost, pattern string) bool {
	repoHost = strings.ToLower(strings.TrimSpace(repoHost))
	pattern = strings.ToLower(strings.TrimSpace(pattern))
	if pattern == "" || pattern == "*" {
		return true
	}
	return repoHost == pattern || strings.HasSuffix(repoHost, "."+pattern)
}
