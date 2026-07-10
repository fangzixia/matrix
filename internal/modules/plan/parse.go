package plan

import (
	"strings"
)

// SectionItems 保存从计划 Markdown 章节提取的列表项。
type SectionItems struct {
	Summary        []string
	ScopeIncluded  []string
	ScopeExcluded  []string
	UserAcceptance []string
	Risks          []string
	Conflicts      []string
	Clarifications []string
}

// ParseSectionItems 从标准计划章节提取列表项。
func ParseSectionItems(content string) SectionItems {
	scopeSection := extractSectionAny(content, "范围")
	clarifications := append(
		extractBullets(extractSectionAny(content, "待确认")),
		extractBullets(extractSectionAny(content, "待优化 / 待澄清"))...,
	)
	userAcceptance := extractBullets(extractSectionAny(content, "用户验收（产品语言）"))
	if len(userAcceptance) == 0 {
		userAcceptance = extractLegacyAcceptance(content)
	}
	return SectionItems{
		Summary:        extractBullets(extractSectionAny(content, "摘要")),
		ScopeIncluded:  extractSubsectionBullets(scopeSection, "本次包含"),
		ScopeExcluded:  extractSubsectionBullets(scopeSection, "本次不包含"),
		UserAcceptance: userAcceptance,
		Risks:          extractBullets(extractSectionAny(content, "风险")),
		Conflicts:      extractBullets(extractSectionAny(content, "冲突与依赖")),
		Clarifications: dedupeStrings(clarifications),
	}
}

func extractSectionAny(content string, headings ...string) string {
	for _, heading := range headings {
		if section := extractSection(content, heading); strings.TrimSpace(section) != "" {
			return section
		}
	}
	return ""
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

func extractSubsectionBullets(section, subheading string) []string {
	lines := strings.Split(section, "\n")
	var buf []string
	inSub := false
	for _, line := range lines {
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, "### ") {
			title := strings.TrimSpace(strings.TrimPrefix(trim, "### "))
			if title == subheading {
				inSub = true
				continue
			}
			if inSub {
				break
			}
		}
		if inSub {
			buf = append(buf, line)
		}
	}
	return extractBullets(strings.Join(buf, "\n"))
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

func extractLegacyAcceptance(content string) []string {
	section := extractSectionAny(content, "验收标准")
	var out []string
	for _, line := range strings.Split(section, "\n") {
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, "- AC-") {
			out = append(out, strings.TrimSpace(strings.TrimPrefix(trim, "- ")))
		}
	}
	return out
}

func dedupeStrings(items []string) []string {
	seen := make(map[string]struct{})
	var out []string
	for _, item := range items {
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
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
