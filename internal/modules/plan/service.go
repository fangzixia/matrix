// Package plan 计划文档（.matrix/PLAN-*.md）索引与列表查询；正文存于工作区文件。
package plan

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"matrix/internal/modules/workspace"
	"matrix/internal/platform/db/models"
)

const (
	planDirMatrix      = ".matrix"
	planDirLegacy      = ".spec/plans"
	planDirLegacyReq   = ".spec/requirements"
	planPrefix         = "PLAN-"
	planPrefixLegacy   = "SPEC-"
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
	Content string    `json:"content,omitempty"`
	RunID   string    `json:"run_id,omitempty"`
}

// List 扫描工作区 .matrix/PLAN-*.md 并合并 DB 索引；正文从磁盘读取。
func (s *Service) List(ctx context.Context, projectID uuid.UUID, repositoryID *uuid.UUID) ([]Item, error) {
	root, err := s.ws.SandboxRoot(ctx, projectID, repositoryID)
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
		full := filepath.Join(root, filepath.FromSlash(rel))
		b, _ := os.ReadFile(full)
		out = append(out, Item{
			Path:    rel,
			Title:   titleOrFromContent(title, string(b)),
			Content: string(b),
		})
	}

	matrixDir := filepath.Join(root, planDirMatrix)
	if st, err := os.Stat(matrixDir); err == nil && st.IsDir() {
		entries, _ := os.ReadDir(matrixDir)
		for _, e := range entries {
			if e.IsDir() || !isPlanFileName(e.Name()) {
				continue
			}
			collectFile(filepath.ToSlash(filepath.Join(planDirMatrix, e.Name())), e.Name())
		}
	}

	appendFromDir := func(dir string) {
		_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			if !strings.HasSuffix(strings.ToLower(d.Name()), ".md") {
				return nil
			}
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return nil
			}
			collectFile(filepath.ToSlash(rel), d.Name())
			return nil
		})
	}
	appendFromDir(filepath.Join(root, planDirLegacy))
	appendFromDir(filepath.Join(root, planDirLegacyReq))

	var rows []models.Plan
	_ = s.db.WithContext(ctx).Where("project_id = ?", projectID).Order("updated_at desc").Find(&rows).Error
	for _, r := range rows {
		if _, ok := seen[r.Path]; ok {
			continue
		}
		seen[r.Path] = struct{}{}
		item := Item{ID: r.ID, Path: r.Path, Title: r.Title}
		if r.RunID != nil {
			item.RunID = r.RunID.String()
		}
		full := filepath.Join(root, filepath.FromSlash(r.Path))
		if b, err := os.ReadFile(full); err == nil {
			item.Content = string(b)
			if item.Title == "" {
				item.Title = titleOrFromContent(filepath.Base(r.Path), item.Content)
			}
		}
		out = append(out, item)
	}
	return out, nil
}

// IndexAfterRun 在 plan 阶段成功后，将计划文件路径写入 DB 索引。
func (s *Service) IndexAfterRun(ctx context.Context, projectID uuid.UUID, repositoryID *uuid.UUID, runID uuid.UUID, filePath, repoRoot string) error {
	path := strings.TrimSpace(filePath)
	if path == "" {
		path = findLatestPlanFile(repoRoot)
	}
	if path == "" {
		return nil
	}
	return s.upsert(ctx, projectID, repositoryID, runID, repoRoot, path)
}

func (s *Service) upsert(ctx context.Context, projectID uuid.UUID, repositoryID *uuid.UUID, runID uuid.UUID, repoRoot, relPath string) error {
	relPath = filepath.ToSlash(strings.TrimSpace(relPath))
	full := filepath.Join(repoRoot, filepath.FromSlash(relPath))
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
		Path: relPath, Title: title, UpdatedAt: now, CreatedAt: now,
	}
	return s.db.WithContext(ctx).Create(&row).Error
}

func isPlanFileName(name string) bool {
	up := strings.ToUpper(name)
	return (strings.HasPrefix(up, planPrefix) || strings.HasPrefix(up, planPrefixLegacy)) &&
		strings.HasSuffix(strings.ToLower(name), ".md")
}

func findLatestPlanFile(repoRoot string) string {
	dir := filepath.Join(repoRoot, planDirMatrix)
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
			best = filepath.ToSlash(filepath.Join(planDirMatrix, e.Name()))
		}
	}
	return best
}

func titleOrFromContent(fallback, content string) string {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "# ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "# "))
		}
	}
	return fallback
}
