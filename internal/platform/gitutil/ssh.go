package gitutil

import (
	"context"
	"fmt"
	"matrix/internal/platform/config"
	"os"
	"os/exec"
	"strings"
	"time"
)

// ResolveSSHKey 按仓库 URL 匹配 Git 访问配置中的私钥路径。
func ResolveSSHKey(git config.GitConfig, gitURL string) string {
	host := HostFromURL(gitURL)
	for _, a := range git.Accesses {
		if MatchHost(host, a.Host) && strings.TrimSpace(a.SSHKeyPath) != "" {
			return strings.TrimSpace(a.SSHKeyPath)
		}
	}
	return strings.TrimSpace(git.SSHKeyPath)
}

// SSHCommandEnv 返回带 GIT_SSH_COMMAND 的环境变量切片。
func SSHCommandEnv(git config.GitConfig, gitURL string) []string {
	key := ResolveSSHKey(git, gitURL)
	if key == "" {
		return nil
	}
	return append(os.Environ(), "GIT_SSH_COMMAND=ssh -i "+key+" -o StrictHostKeyChecking=no")
}

// TestConnection 使用 git ls-remote 测试仓库可达性。
func TestConnection(ctx context.Context, git config.GitConfig, gitURL string, timeout time.Duration) (string, error) {
	gitURL = strings.TrimSpace(gitURL)
	if gitURL == "" {
		return "", fmt.Errorf("git_url 不能为空")
	}
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(cctx, "git", "ls-remote", "--heads", gitURL)
	if env := SSHCommandEnv(git, gitURL); env != nil {
		cmd.Env = env
	}
	out, err := cmd.CombinedOutput()
	text := strings.TrimSpace(string(out))
	if err != nil {
		if text == "" {
			return "", fmt.Errorf("连接失败: %w", err)
		}
		return "", fmt.Errorf("连接失败: %w: %s", err, text)
	}
	lines := strings.Split(text, "\n")
	return fmt.Sprintf("连接成功，发现 %d 个分支引用", len(lines)), nil
}
