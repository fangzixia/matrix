package plan

import (
	"strings"
)

// SectionItems 保存从计划 Markdown 章节提取的列表项。
type SectionItems struct {
	Risks          []string
	Conflicts      []string
	Clarifications []string
}

// ParseSectionItems 从标准计划章节提取列表项。
func ParseSectionItems(content string) SectionItems {
	return SectionItems{
		Risks:          extractBullets(extractSection(content, "风险")),
		Conflicts:      extractBullets(extractSection(content, "冲突与依赖")),
		Clarifications: extractBullets(extractSection(content, "待优化 / 待澄清")),
	}
}

// extractSection 从计划 Markdown 提取指定章节正文。
func extractSection(content, heading string) string {
	lines := strings.Split(content, "\n")
	var buf []string
	inSection := false
	for _, line := range lines {
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, "## ") {
			title := strings.TrimSpace(strings.TrimPrefix(trim, "## "))
			if title == heading {
				inSection = true
				continue
			}
			if inSection {
				break
			}
		}
		if inSection {
			buf = append(buf, line)
		}
	}
	return strings.Join(buf, "\n")
}

// extractBullets 从章节文本提取列表项。
func extractBullets(section string) []string {
	var out []string
	for _, line := range strings.Split(section, "\n") {
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, "- ") {
			item := strings.TrimSpace(strings.TrimPrefix(trim, "- "))
			if item != "" && !isNoneMarker(item) {
				out = append(out, item)
			}
		}
	}
	return out
}

// isNoneMarker 判断列表项是否为「无/暂无」占位标记。
func isNoneMarker(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "无", "暂无", "none", "n/a", "-":
		return true
	default:
		return false
	}
}
