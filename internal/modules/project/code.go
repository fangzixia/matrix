package project

import (
	"fmt"
	"regexp"
	"strings"
)

var projectCodePattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,62}[a-z0-9])?$`)

// NormalizeProjectCode 转小写并剔除不适合文件夹命名的非法字符。
func NormalizeProjectCode(code string) string {
	code = strings.ToLower(strings.TrimSpace(code))
	code = strings.Trim(code, "-.")
	if code == "" {
		return ""
	}
	var b strings.Builder
	for _, r := range code {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-' || r == '_' || r == '.':
			b.WriteRune('-')
		}
	}
	out := strings.Trim(b.String(), "-")
	for strings.Contains(out, "--") {
		out = strings.ReplaceAll(out, "--", "-")
	}
	if len(out) > 64 {
		out = out[:64]
		out = strings.Trim(out, "-")
	}
	return out
}

// ValidateProjectCode 校验符合文件夹安全规则的项目编码格式。
func ValidateProjectCode(code string) error {
	code = NormalizeProjectCode(code)
	if code == "" {
		return fmt.Errorf("项目编码不能为空")
	}
	if !projectCodePattern.MatchString(code) {
		return fmt.Errorf("项目编码仅允许小写字母、数字与连字符，且不能以连字符开头或结尾")
	}
	return nil
}
