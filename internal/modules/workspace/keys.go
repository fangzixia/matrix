package workspace

import (
	"context"

	"github.com/google/uuid"
)

// ProjectKeyResolver 将项目 ID 解析为工作区目录键（项目编码）。
type ProjectKeyResolver interface {
	ProjectWorkspaceKey(ctx context.Context, projectID uuid.UUID) (string, error)
}
