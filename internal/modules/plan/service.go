// Package plan 计划文档（docs/plans/PLAN-*.md）索引与列表查询；正文存于项目 docs 目录。
package plan

import (
	"context"
	"matrix/internal/modules/workspace"
	"matrix/internal/platform/db/models"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	planPrefix = "PLAN-"
)

// Service 管理计划文档索引与列表查询。
type Service struct {
	db *gorm.DB
	ws *workspace.Service
}

// NewService 创建计划文档服务实例。
func NewService(db *gorm.DB, ws *workspace.Service) *Service {
	return &Service{db: db, ws: ws}
}

// Item 是计划文档列表项 API 返回的数据传输对象。
type Item struct {
	ID      uuid.UUID `json:"id,omitempty"`
	Path    string    `json:"path"`
	Title   string    `json:"title"`
	Status  string    `json:"status,omitempty"`
	Content string    `json:"content,omitempty"`
	RunID   string    `json:"run_id,omitempty"`
}

// List 扫描 docs/plans/ 并合并 DB 索引；正文从磁盘读取。
func (s *Service) List(ctx context.Context, projectID uuid.UUID, repositoryID *uuid.UUID) ([]Item, error) {
	if err := s.ws.EnsureDocsLayout(projectID); err != nil {
		return nil, err
	}
	docsRoot, err := s.ws.DocsRoot(projectID)
	if err != nil {
		return nil, err
	}
	seen := make(map[string]struct{})
	var out []Item
	collectFile := func(rel string, title string) {
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
			Path:    rel,
			Title:   titleOrFromContent(title, string(b)),
			Content: string(b),
		})
	}
	statusFor := func(rel string) string {
		st, err := s.PlanStatus(ctx, projectID, rel)
		if err != nil || st == "" {
			return StatusDraft
		}
		return st
	}
	plansDir := filepath.Join(docsRoot, "plans")
	if st, err := os.Stat(plansDir); err == nil && st.IsDir() {
		entries, _ := os.ReadDir(plansDir)
		for _, e := range entries {
			if e.IsDir() || !isPlanFileName(e.Name()) {
				continue
			}
			collectFile(filepath.ToSlash(filepath.Join(workspace.DocsPlansRel, e.Name())), e.Name())
		}
	}
	for i := range out {
		out[i].Status = statusFor(out[i].Path)
	}
	var rows []models.Plan
	_ = s.db.WithContext(ctx).Where("project_id = ?", projectID).Order("updated_at desc").Find(&rows).Error
	for _, r := range rows {
		rel, err := workspace.SanitizeDocLogicalPath(r.Path)
		if err != nil || rel == "" {
			continue
		}
		if _, ok := seen[rel]; ok {
			continue
		}
		seen[rel] = struct{}{}
		item := Item{ID: r.ID, Path: rel, Title: r.Title, Status: r.Status}
		if item.Status == "" {
			item.Status = StatusDraft
		}
		if r.RunID != nil {
			item.RunID = r.RunID.String()
		}
		if full, err := s.ws.ResolveDocPath(projectID, rel); err == nil {
			if b, err := os.ReadFile(full); err == nil {
				item.Content = string(b)
				if item.Title == "" {
					item.Title = titleOrFromContent(filepath.Base(rel), item.Content)
				}
			}
		}
		out = append(out, item)
	}
	return out, nil
}

// IndexAfterRun 在 plan 阶段成功后，将计划文件路径写入 DB 索引。
func (s *Service) IndexAfterRun(ctx context.Context, projectID uuid.UUID, repositoryID *uuid.UUID, runID uuid.UUID, filePath, docsRoot string) error {
	path, err := workspace.SanitizeDocLogicalPath(strings.TrimSpace(filePath))
	if err != nil {
		return err
	}
	if path == "" {
		path = findLatestPlanFile(docsRoot)
	}
	if path == "" {
		return nil
	}
	return s.upsert(ctx, projectID, repositoryID, runID, path)
}

// upsert 插入或更新数据库索引行。
func (s *Service) upsert(ctx context.Context, projectID uuid.UUID, repositoryID *uuid.UUID, runID uuid.UUID, relPath string) error {
	relPath, err := workspace.SanitizeDocLogicalPath(relPath)
	if err != nil {
		return err
	}
	full, err := s.ws.ResolveDocPath(projectID, relPath)
	if err != nil {
		return err
	}
	b, err := os.ReadFile(full)
	if err != nil {
		return err
	}
	title := titleOrFromContent(filepath.Base(relPath), string(b))
	now := time.Now()
	var existing models.Plan
	q := s.db.WithContext(ctx).Where("project_id = ? AND path = ?", projectID, relPath)
	if repositoryID != nil {
		q = q.Where("repository_id = ? OR repository_id IS NULL", *repositoryID)
	}
	err = q.First(&existing).Error
	if err == nil {
		existing.Title = title
		existing.RunID = &runID
		existing.UpdatedAt = now
		if repositoryID != nil {
			existing.RepositoryID = repositoryID
		}
		return s.db.WithContext(ctx).Save(&existing).Error
	}
	row := models.Plan{
		ProjectID: projectID, RepositoryID: repositoryID, RunID: &runID,
		Path: relPath, Title: title, Status: StatusDraft, UpdatedAt: now, CreatedAt: now,
	}
	return s.db.WithContext(ctx).Create(&row).Error
}

// isPlanFileName 判断文件名是否为计划文档。
func isPlanFileName(name string) bool {
	up := strings.ToUpper(name)
	return strings.HasPrefix(up, planPrefix) && strings.HasSuffix(strings.ToLower(name), ".md")
}

// findLatestPlanFile 在文档目录中查找最新计划文件。
func findLatestPlanFile(docsRoot string) string {
	dir := filepath.Join(docsRoot, "plans")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	var best string
	var bestTime time.Time
	for _, e := range entries {
		if e.IsDir() || !isPlanFileName(e.Name()) {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.ModTime().After(bestTime) {
			bestTime = info.ModTime()
			best = filepath.ToSlash(filepath.Join(workspace.DocsPlansRel, e.Name()))
		}
	}
	return best
}

// titleOrFromContent 从内容提取标题或使用文件名。
func titleOrFromContent(fallback, content string) string {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "# ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "# "))
		}
	}
	return fallback
}
