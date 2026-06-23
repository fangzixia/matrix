package audit

import (
	"bufio"
	"encoding/json"
	"fmt"
	"matrix/internal/platform/storage"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ListSessions 返回最近会话索引列表（按时间倒序）。
func ListSessions(sessionsDir string, limit int) ([]SessionIndex, error) {
	dir := sessionsDir
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []SessionIndex
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".meta.json") {
			continue
		}
		id := strings.TrimSuffix(e.Name(), ".meta.json")
		metaPath := filepath.Join(dir, e.Name())
		data, err := os.ReadFile(metaPath)
		if err != nil {
			continue
		}
		var meta SessionMeta
		if json.Unmarshal(data, &meta) != nil {
			continue
		}
		out = append(out, SessionIndex{
			SessionID:  id,
			StartedAt:  meta.StartedAt,
			EndedAt:    meta.EndedAt,
			StopReason: meta.StopReason,
			TurnCount:  meta.TurnCount,
			Path:       filepath.Join(dir, id+".jsonl"),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		return timeRFC3339After(out[i].StartedAt, out[j].StartedAt)
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// ReadSession 加载指定会话的 meta 与事件。
func ReadSession(sessionsDir, sessionID string, opts ExportOptions) (ExportBundle, error) {
	if sessionsDir == "" || sessionID == "" {
		return ExportBundle{}, fmt.Errorf("audit: sessions dir or session_id empty")
	}
	dir := sessionsDir
	metaPath := filepath.Join(dir, sessionID+".meta.json")
	jsonlPath := filepath.Join(dir, sessionID+".jsonl")
	var meta SessionMeta
	if data, err := os.ReadFile(metaPath); err == nil {
		_ = json.Unmarshal(data, &meta)
	}
	meta.SessionID = sessionID
	events, err := readJSONLEvents(jsonlPath, opts.MaxEvents)
	if err != nil {
		return ExportBundle{}, err
	}
	return ExportBundle{
		Meta:      meta,
		Events:    events,
		JSONLPath: jsonlPath,
		MetaPath:  metaPath,
		Subagents: storage.ProjectSubagentsDir(sessionsDir),
	}, nil
}

// readJSONLEvents 从 JSONL 文件读取审计事件列表。
func readJSONLEvents(path string, maxEvents int) ([]Event, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()
	var all []Event
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var ev Event
		if json.Unmarshal([]byte(line), &ev) != nil {
			continue
		}
		all = append(all, ev)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if maxEvents > 0 && len(all) > maxEvents {
		all = all[len(all)-maxEvents:]
	}
	return all, nil
}

// FormatForLLM 渲染 Markdown 时间线，便于粘贴给 LLM 分析。
func FormatForLLM(bundle ExportBundle) string {
	var b strings.Builder
	m := bundle.Meta
	b.WriteString("# Matrix Session Diagnostic\n\n")
	b.WriteString("## Summary\n\n")
	b.WriteString(fmt.Sprintf("- **session_id**: %s\n", m.SessionID))
	if m.StartedAt != "" {
		b.WriteString(fmt.Sprintf("- **started_at**: %s\n", m.StartedAt))
	}
	if m.EndedAt != "" {
		b.WriteString(fmt.Sprintf("- **ended_at**: %s\n", m.EndedAt))
	}
	if m.Model != "" {
		b.WriteString(fmt.Sprintf("- **model**: %s\n", m.Model))
	}
	if m.Workspace != "" {
		b.WriteString(fmt.Sprintf("- **workspace**: %s\n", m.Workspace))
	}
	if m.TaskPreview != "" {
		b.WriteString(fmt.Sprintf("- **task_preview**: %s\n", m.TaskPreview))
	}
	if m.StopReason != "" {
		b.WriteString(fmt.Sprintf("- **stop_reason**: %s\n", m.StopReason))
	}
	if m.TurnCount > 0 {
		b.WriteString(fmt.Sprintf("- **turn_count**: %d\n", m.TurnCount))
	}
	if m.DurationMs > 0 {
		b.WriteString(fmt.Sprintf("- **duration_ms**: %d\n", m.DurationMs))
	}
	if m.Error != "" {
		b.WriteString(fmt.Sprintf("- **error**: %s\n", m.Error))
	}
	if bundle.JSONLPath != "" {
		b.WriteString(fmt.Sprintf("- **jsonl_path**: %s\n", bundle.JSONLPath))
	}
	if bundle.Subagents != "" {
		b.WriteString(fmt.Sprintf("- **subagents_dir**: %s (per-agent *.jsonl sidechains)\n", bundle.Subagents))
	}
	b.WriteString("\n## Timeline\n\n")
	for _, ev := range bundle.Events {
		line := fmt.Sprintf("`%s` **%s**", ev.Ts, ev.Event)
		if ev.Turn > 0 {
			line += fmt.Sprintf(" turn=%d", ev.Turn)
		}
		if ev.Component != "" {
			line += fmt.Sprintf(" component=%s", ev.Component)
		}
		if ev.AgentID != "" {
			line += fmt.Sprintf(" agent=%s", ev.AgentID)
		}
		if ev.ToolUseID != "" {
			line += fmt.Sprintf(" tool_use_id=%s", ev.ToolUseID)
		}
		b.WriteString(line + "\n")
		if len(ev.Data) > 0 {
			for k, v := range ev.Data {
				b.WriteString(fmt.Sprintf("  - %s: %v\n", k, v))
			}
		}
		b.WriteString("\n")
	}
	b.WriteString("## Notes for analysis\n\n")
	b.WriteString("- Use stop_reason and error to diagnose failures.\n")
	b.WriteString("- Review turn.tool_call / turn.tool_result for tool-related issues.\n")
	b.WriteString("- context.compact events show when history was compressed.\n")
	b.WriteString("- For full sub-agent streams, inspect files under subagents_dir.\n")
	return b.String()
}

// WriteExportFiles 写入 LLM Markdown，并可选将 JSONL 复制到 destDir。
func WriteExportFiles(bundle ExportBundle, destDir string) (mdPath, jsonlCopy string, err error) {
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return "", "", err
	}
	id := bundle.Meta.SessionID
	if id == "" {
		id = "unknown"
	}
	mdPath = filepath.Join(destDir, id+"-diagnostic.md")
	if err := os.WriteFile(mdPath, []byte(FormatForLLM(bundle)), 0o644); err != nil {
		return "", "", err
	}
	if bundle.JSONLPath != "" {
		data, err := os.ReadFile(bundle.JSONLPath)
		if err == nil {
			jsonlCopy = filepath.Join(destDir, id+".jsonl")
			if werr := os.WriteFile(jsonlCopy, data, 0o644); werr != nil {
				return mdPath, "", werr
			}
		}
	}
	return mdPath, jsonlCopy, nil
}

// ToDiagnosticDTO 将数据包转换为 Bridge API 用的 DiagnosticDTO（默认保留末尾 200 条事件）。
func ToDiagnosticDTO(bundle ExportBundle, maxTail int) DiagnosticDTO {
	events := bundle.Events
	if maxTail <= 0 {
		maxTail = 200
	}
	if len(events) > maxTail {
		events = events[len(events)-maxTail:]
	}
	return DiagnosticDTO{
		SessionID:   bundle.Meta.SessionID,
		Meta:        bundle.Meta,
		EventsTail:  events,
		LLMMarkdown: FormatForLLM(bundle),
		JSONLPath:   bundle.JSONLPath,
	}
}
