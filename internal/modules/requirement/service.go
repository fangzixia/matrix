// Package requirement 需求文档管理与列表查询。
package requirement

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
	specDirMatrix = ".matrix"
	specDirLegacy = ".spec/requirements"
)

// Service 管理需求文档的列表查询与内容读取。
type Service struct {
	db *gorm.DB
	ws *workspace.Service
}

// NewService 创建需求文档服务实例。
func NewService(db *gorm.DB, ws *workspace.Service) *Service {
	return &Service{db: db, ws: ws}
}

// Item 是需求文档列表项 API 返回的数据传输对象。
type Item struct {
	ID      uuid.UUID `json:"id"`
	Path    string    `json:"path"`
	Title   string    `json:"title"`
	Content string    `json:"content,omitempty"`
}

// List 返回列表。
func (s *Service) List(ctx context.Context, projectID uuid.UUID, repositoryID *uuid.UUID) ([]Item, error) {
	root, err := s.ws.SandboxRoot(ctx, projectID, repositoryID)
	if err != nil {
		return nil, err
	}
	seen := make(map[string]struct{})
	var out []Item

	appendFromDir := func(dir string, prefix string) {
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
			rel = filepath.ToSlash(rel)
			if _, ok := seen[rel]; ok {
				return nil
			}
			seen[rel] = struct{}{}
			b, _ := os.ReadFile(path)
			title := d.Name()
			if prefix != "" {
				title = prefix + title
			}
			out = append(out, Item{Path: rel, Title: title, Content: string(b)})
			return nil
		})
	}

	matrixDir := filepath.Join(root, specDirMatrix)
	if st, err := os.Stat(matrixDir); err == nil && st.IsDir() {
		entries, _ := os.ReadDir(matrixDir)
		for _, e := range entries {
			if e.IsDir() || !strings.HasPrefix(e.Name(), "SPEC-") || !strings.HasSuffix(e.Name(), ".md") {
				continue
			}
			full := filepath.Join(matrixDir, e.Name())
			rel := filepath.ToSlash(filepath.Join(specDirMatrix, e.Name()))
			if _, ok := seen[rel]; ok {
				continue
			}
			seen[rel] = struct{}{}
			b, _ := os.ReadFile(full)
			out = append(out, Item{Path: rel, Title: e.Name(), Content: string(b)})
		}
	}

	legacyDir := filepath.Join(root, specDirLegacy)
	appendFromDir(legacyDir, "")

	var rows []models.Requirement
	_ = s.db.WithContext(ctx).Where("project_id = ?", projectID).Find(&rows).Error
	for _, r := range rows {
		if _, ok := seen[r.Path]; ok {
			continue
		}
		seen[r.Path] = struct{}{}
		out = append(out, Item{ID: r.ID, Path: r.Path, Title: r.Title, Content: r.Content})
	}
	return out, nil
}
