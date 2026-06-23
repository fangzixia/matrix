// Package eval 解析评测报告 Markdown（docs/evaluations/EVAL-*.md）。
package eval

import (
	"fmt"
	"matrix/internal/modules/workspace"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

const PassScore = 8.0

var scorePatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)综合分[：:]\s*([0-9]+(?:\.[0-9]+)?)\s*(?:/\s*10)?`),
	regexp.MustCompile(`(?i)overall\s*score[：:]\s*([0-9]+(?:\.[0-9]+)?)`),
}

// ParseScore 从评测报告内容中提取综合分。
func ParseScore(content string) (float64, bool) {
	for _, re := range scorePatterns {
		if m := re.FindStringSubmatch(content); len(m) >= 2 {
			v, err := strconv.ParseFloat(m[1], 64)
			if err == nil {
				return v, true
			}
		}
	}
	return 0, false
}

// LatestEval 返回 docs/evaluations 下最新的 EVAL-*.md 逻辑路径。
func LatestEval(docsRoot string, planLogicalPath string) (logicalPath string, content string, err error) {
	dir := filepath.Join(docsRoot, "evaluations")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return "", "", fmt.Errorf("未找到评测报告")
		}
		return "", "", err
	}
	planBase := strings.TrimSuffix(filepath.Base(planLogicalPath), filepath.Ext(planLogicalPath))
	type candidate struct {
		rel     string
		modTime time.Time
	}
	var cands []candidate
	for _, e := range entries {
		if e.IsDir() || !strings.HasPrefix(strings.ToUpper(e.Name()), "EVAL-") || !strings.HasSuffix(strings.ToLower(e.Name()), ".md") {
			continue
		}
		if planBase != "" && !strings.Contains(strings.ToUpper(e.Name()), strings.ToUpper(planBase)) {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		rel := filepath.ToSlash(filepath.Join(workspace.DocsEvaluationsRel, e.Name()))
		cands = append(cands, candidate{rel: rel, modTime: info.ModTime()})
	}
	if len(cands) == 0 {
		return "", "", fmt.Errorf("未找到评测报告")
	}
	sort.Slice(cands, func(i, j int) bool {
		return cands[i].modTime.After(cands[j].modTime)
	})
	rel := cands[0].rel
	full := filepath.Join(docsRoot, "evaluations", filepath.Base(rel))
	b, err := os.ReadFile(full)
	if err != nil {
		return "", "", err
	}
	return rel, string(b), nil
}

// LatestScore 读取最新匹配的评测报告并解析其综合分。
func LatestScore(docsRoot, planLogicalPath string) (score float64, evalPath string, ok bool, err error) {
	rel, content, err := LatestEval(docsRoot, planLogicalPath)
	if err != nil {
		return 0, "", false, err
	}
	score, ok = ParseScore(content)
	if !ok {
		return 0, rel, false, fmt.Errorf("评测报告未包含可解析的综合分: %s", rel)
	}
	return score, rel, true, nil
}
