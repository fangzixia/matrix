// Package artifact 评测与构建产物记录。
package artifact

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"matrix/internal/modules/workspace"
	"matrix/internal/platform/db/models"
)

const (
	evalDirMatrix = ".matrix"
	evalDirLegacy = ".spec/evaluations"
)

// Service 管理评测与构建产物的列表查询。
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
	ID    uuid.UUID `json:"id"`
	Kind  string    `json:"kind"`
	Path  string    `json:"path"`
	Title string    `json:"title"`
}

// ListEvaluations 返回评测产物列表。
func (s *Service) ListEvaluations(ctx context.Context, projectID uuid.UUID, repositoryID *uuid.UUID) ([]Item, error) {
	root, err := s.ws.SandboxRoot(ctx, projectID, repositoryID)
	if err != nil {
		return nil, err
	}
	seen := make(map[string]struct{})
	var out []Item

	matrixDir := filepath.Join(root, evalDirMatrix)
	if st, err := os.Stat(matrixDir); err == nil && st.IsDir() {
		entries, _ := os.ReadDir(matrixDir)
		for _, e := range entries {
			name := e.Name()
			if e.IsDir() || !strings.HasPrefix(name, "EVAL-") || !strings.HasSuffix(name, ".md") {
				continue
			}
			rel := filepath.ToSlash(filepath.Join(evalDirMatrix, name))
			if _, ok := seen[rel]; ok {
				continue
			}
			seen[rel] = struct{}{}
			out = append(out, Item{Kind: "evaluation", Path: rel, Title: name})
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
		rel = filepath.ToSlash(rel)
		if _, ok := seen[rel]; ok {
			return nil
		}
		seen[rel] = struct{}{}
		out = append(out, Item{Kind: "evaluation", Path: rel, Title: filepath.Base(path)})
		return nil
	})

	var rows []models.Artifact
	_ = s.db.WithContext(ctx).Where("project_id = ? AND kind = ?", projectID, "evaluation").Find(&rows).Error
	for _, r := range rows {
		if _, ok := seen[r.Path]; ok {
			continue
		}
		seen[r.Path] = struct{}{}
		out = append(out, Item{ID: r.ID, Kind: r.Kind, Path: r.Path, Title: r.Path})
	}
	return out, nil
}
