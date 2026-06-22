// Package artifact 评测与构建产物（.matrix/EVAL-PLAN-*.md）索引与列表查询。
package artifact

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
	evalDirMatrix    = ".matrix"
	evalDirLegacy    = ".spec/evaluations"
	evalPrefix       = "EVAL-"
)

// Service 管理评测与构建产物索引。
type Service struct {
	db *gorm.DB
	ws *workspace.Service
}

// NewService 创建产物服务实例。
func NewService(db *gorm.DB, ws *workspace.Service) *Service {
	return &Service{db: db, ws: ws}
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

// ListEvaluations 返回评测产物列表（正文从磁盘读取）。
func (s *Service) ListEvaluations(ctx context.Context, projectID uuid.UUID, repositoryID *uuid.UUID) ([]Item, error) {
	root, err := s.ws.SandboxRoot(ctx, projectID, repositoryID)
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
		full := filepath.Join(root, filepath.FromSlash(rel))
		b, _ := os.ReadFile(full)
		out = append(out, Item{
			Kind: "evaluation", Path: rel, Title: title,
			Content: string(b),
		})
	}

	matrixDir := filepath.Join(root, evalDirMatrix)
	if st, err := os.Stat(matrixDir); err == nil && st.IsDir() {
		entries, _ := os.ReadDir(matrixDir)
		for _, e := range entries {
			name := e.Name()
			if e.IsDir() || !isEvalFileName(name) {
				continue
			}
			collectFile(filepath.ToSlash(filepath.Join(evalDirMatrix, name)), name)
		}
	}

	legacyDir := filepath.Join(root, evalDirLegacy)
	_ = filepath.WalkDir(legacyDir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}
		collectFile(filepath.ToSlash(rel), filepath.Base(path))
		return nil
	})

	var rows []models.Artifact
	_ = s.db.WithContext(ctx).Where("project_id = ? AND kind = ?", projectID, "evaluation").
		Order("created_at desc").Find(&rows).Error
	for _, r := range rows {
		if _, ok := seen[r.Path]; ok {
			continue
		}
		seen[r.Path] = struct{}{}
		item := Item{
			ID: r.ID, Kind: r.Kind, Path: r.Path,
			PlanPath: r.PlanPath, Title: r.Title,
		}
		if item.Title == "" {
			item.Title = r.Path
		}
		if r.RunID != nil {
			item.RunID = r.RunID.String()
		}
		full := filepath.Join(root, filepath.FromSlash(r.Path))
		if b, err := os.ReadFile(full); err == nil {
			item.Content = string(b)
		}
		out = append(out, item)
	}
	return out, nil
}

// IndexAfterRun 在 verify/build 阶段成功后，将评测文件路径写入 DB 索引。
func (s *Service) IndexAfterRun(ctx context.Context, projectID uuid.UUID, repositoryID *uuid.UUID, runID uuid.UUID, planPath, repoRoot string) error {
	evalPath := findLatestEvalFile(repoRoot)
	if evalPath == "" {
		return nil
	}
	return s.upsert(ctx, projectID, repositoryID, runID, evalPath, planPath, repoRoot)
}

func (s *Service) upsert(ctx context.Context, projectID uuid.UUID, repositoryID *uuid.UUID, runID uuid.UUID, evalPath, planPath, repoRoot string) error {
	evalPath = filepath.ToSlash(strings.TrimSpace(evalPath))
	title := filepath.Base(evalPath)
	if planPath != "" {
		planPath = filepath.ToSlash(strings.TrimSpace(planPath))
	}

	var existing models.Artifact
	q := s.db.WithContext(ctx).Where("project_id = ? AND path = ?", projectID, evalPath)
	err := q.First(&existing).Error
	if err == nil {
		existing.RunID = &runID
		existing.PlanPath = planPath
		existing.Title = title
		if repositoryID != nil {
			existing.RepositoryID = repositoryID
		}
		return s.db.WithContext(ctx).Save(&existing).Error
	}

	row := models.Artifact{
		ProjectID: projectID, RepositoryID: repositoryID, RunID: &runID,
		Kind: "evaluation", Path: evalPath, PlanPath: planPath, Title: title,
		CreatedAt: time.Now(),
	}
	return s.db.WithContext(ctx).Create(&row).Error
}

func isEvalFileName(name string) bool {
	up := strings.ToUpper(name)
	return strings.HasPrefix(up, evalPrefix) && strings.HasSuffix(strings.ToLower(name), ".md")
}

func findLatestEvalFile(repoRoot string) string {
	dir := filepath.Join(repoRoot, evalDirMatrix)
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
			best = filepath.ToSlash(filepath.Join(evalDirMatrix, e.Name()))
		}
	}
	return best
}
