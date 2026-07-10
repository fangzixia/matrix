// Package artifact 评测与构建产物（docs/evaluations/EVAL-*.md）索引与列表查询。
package artifact

import (
	"context"
	"matrix/internal/modules/docmeta"
	"matrix/internal/modules/workspace"
	"matrix/internal/platform/db/repo"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

const evalPrefix = "EVAL-"

// Service 管理评测与构建产物索引。
type Service struct {
	stores *repo.Stores
	ws     *workspace.Service
}

// NewService 创建产物服务实例。
func NewService(stores *repo.Stores, ws *workspace.Service) *Service {
	return &Service{stores: stores, ws: ws}
}

// Item 是产物列表项 API 返回的数据传输对象。
type Item struct {
	ID       uuid.UUID `json:"id,omitempty"`
	Kind     string    `json:"kind"`
	Path     string    `json:"path"`
	PlanPath string    `json:"plan_path,omitempty"`
	Title    string    `json:"title"`
	Content  string    `json:"content,omitempty"`
	RunID    string    `json:"run_id,omitempty"`
}

// ListEvaluations 返回评测产物列表（正文从 docs 目录读取）。
func (s *Service) ListEvaluations(ctx context.Context, projectID uuid.UUID, repositoryID *uuid.UUID) ([]Item, error) {
	if err := s.ws.EnsureDocsLayout(projectID); err != nil {
		return nil, err
	}
	docsRoot, err := s.ws.DocsRoot(projectID)
	if err != nil {
		return nil, err
	}
	seen := make(map[string]struct{})
	var out []Item
	collectFile := func(rel, title string) {
		if _, ok := seen[rel]; ok {
			return
		}
		seen[rel] = struct{}{}
		full, err := s.ws.ResolveDocPath(projectID, rel)
		if err != nil {
			return
		}
		b, _ := os.ReadFile(full)
		out = append(out, Item{
			Kind: "evaluation", Path: rel,
			Title:   docmeta.TitleOrFallback(title, string(b)),
			Content: string(b),
		})
	}
	evalsDir := filepath.Join(docsRoot, "evaluations")
	if st, err := os.Stat(evalsDir); err == nil && st.IsDir() {
		entries, _ := os.ReadDir(evalsDir)
		for _, e := range entries {
			name := e.Name()
			if e.IsDir() || !isEvalFileName(name) {
				continue
			}
			collectFile(filepath.ToSlash(filepath.Join(workspace.DocsEvaluationsRel, name)), name)
		}
	}
	rows, _ := s.stores.Artifact.ListEvaluations(ctx, projectID)
	for _, r := range rows {
		rel, err := workspace.SanitizeDocLogicalPath(r.Path)
		if err != nil || rel == "" {
			continue
		}
		if _, ok := seen[rel]; ok {
			continue
		}
		seen[rel] = struct{}{}
		planPath, _ := workspace.SanitizeDocLogicalPath(r.PlanPath)
		item := Item{
			ID: r.ID, Kind: r.Kind, Path: rel,
			PlanPath: planPath, Title: r.Title,
		}
		if item.Title == "" {
			item.Title = rel
		}
		if r.RunID != nil {
			item.RunID = r.RunID.String()
		}
		if full, err := s.ws.ResolveDocPath(projectID, rel); err == nil {
			if b, err := os.ReadFile(full); err == nil {
				item.Content = string(b)
				if item.Title == "" || item.Title == rel {
					item.Title = docmeta.TitleOrFallback(filepath.Base(rel), item.Content)
				}
			}
		}
		out = append(out, item)
	}
	return out, nil
}

// IndexAfterRun 在 verify 阶段成功后，将评测文件路径写入 DB 索引。
func (s *Service) IndexAfterRun(ctx context.Context, projectID uuid.UUID, repositoryID *uuid.UUID, runID uuid.UUID, planPath, docsRoot string) error {
	evalPath := findLatestEvalFile(docsRoot)
	if evalPath == "" {
		return nil
	}
	planPath, err := workspace.SanitizeDocLogicalPath(planPath)
	if err != nil {
		return err
	}
	return s.upsert(ctx, projectID, repositoryID, runID, evalPath, planPath)
}

// upsert 插入或更新数据库索引行。
func (s *Service) upsert(ctx context.Context, projectID uuid.UUID, repositoryID *uuid.UUID, runID uuid.UUID, evalPath, planPath string) error {
	evalPath, err := workspace.SanitizeDocLogicalPath(evalPath)
	if err != nil {
		return err
	}
	title := filepath.Base(evalPath)
	if full, err := s.ws.ResolveDocPath(projectID, evalPath); err == nil {
		if b, err := os.ReadFile(full); err == nil {
			title = docmeta.TitleOrFallback(title, string(b))
		}
	}
	return s.stores.Artifact.IndexAfterRun(ctx, repo.ArtifactIndexParams{
		ProjectID: projectID, RepositoryID: repositoryID, RunID: runID,
		Path: evalPath, PlanPath: planPath, Title: title,
	})
}

// isEvalFileName 判断文件名是否为评测报告。
func isEvalFileName(name string) bool {
	up := strings.ToUpper(name)
	return strings.HasPrefix(up, evalPrefix) && strings.HasSuffix(strings.ToLower(name), ".md")
}

// findLatestEvalFile 在文档目录中查找最新评测文件。
func findLatestEvalFile(docsRoot string) string {
	dir := filepath.Join(docsRoot, "evaluations")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	var best string
	var bestTime time.Time
	for _, e := range entries {
		if e.IsDir() || !isEvalFileName(e.Name()) {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.ModTime().After(bestTime) {
			bestTime = info.ModTime()
			best = filepath.ToSlash(filepath.Join(workspace.DocsEvaluationsRel, e.Name()))
		}
	}
	return best
}
