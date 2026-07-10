// Package workspace Git 工作区：Run 级仓库克隆、文档目录与文件访问。
package workspace

import (
	"context"
	"fmt"
	"matrix/internal/modules/project"
	"matrix/internal/modules/repository"
	"matrix/internal/modules/settings"
	"matrix/internal/platform/storage"

	"github.com/google/uuid"
)

// Service 管理 Run 级 Git 工作区与项目文档目录。
type Service struct {
	paths    storage.Paths
	settings *settings.Service
	repos    *repository.Service
	keys     *project.Service
}

// NewService 创建工作区服务实例。
func NewService(paths storage.Paths, settings *settings.Service, repos *repository.Service) *Service {
	return &Service{paths: paths, settings: settings, repos: repos}
}

// SetProjectKeyResolver 注入项目工作区目录键解析器。
func (s *Service) SetProjectKeyResolver(r *project.Service) {
	s.keys = r
}

// ProjectWorkspaceKey 返回项目工作区目录键（项目编码）。
func (s *Service) ProjectWorkspaceKey(ctx context.Context, projectID uuid.UUID) (string, error) {
	if s.keys == nil {
		return "", fmt.Errorf("workspace: project key resolver not configured")
	}
	return s.keys.ProjectWorkspaceKey(ctx, projectID)
}

// FileEntry 是工作区文件树节点 API 返回的数据传输对象。
type FileEntry struct {
	Name  string `json:"name"`
	Path  string `json:"path"`
	IsDir bool   `json:"is_dir"`
	Size  int64  `json:"size"`
}
