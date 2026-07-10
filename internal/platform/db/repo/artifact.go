package repo

import (
	"context"

	"matrix/internal/platform/db/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ArtifactRepo 封装 Artifact 表持久化操作。
type ArtifactRepo struct {
	db *gorm.DB
}

// NewArtifactRepo 创建 ArtifactRepo。
func NewArtifactRepo(db *gorm.DB) *ArtifactRepo {
	return &ArtifactRepo{db: db}
}

// ListByProjectAndKind 按项目与类型列出产物索引。
func (r *ArtifactRepo) ListByProjectAndKind(ctx context.Context, projectID uuid.UUID, kind string) ([]models.Artifact, error) {
	var rows []models.Artifact
	if err := r.db.WithContext(ctx).Where("project_id = ? AND kind = ?", projectID, kind).
		Order("created_at desc").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// GetByProjectAndPath 按项目与路径查询。
func (r *ArtifactRepo) GetByProjectAndPath(ctx context.Context, projectID uuid.UUID, path string) (*models.Artifact, error) {
	var existing models.Artifact
	if err := r.db.WithContext(ctx).Where("project_id = ? AND path = ?", projectID, path).First(&existing).Error; err != nil {
		return nil, err
	}
	return &existing, nil
}

// Save 保存产物索引。
func (r *ArtifactRepo) Save(ctx context.Context, row *models.Artifact) error {
	return r.db.WithContext(ctx).Save(row).Error
}

// Create 创建产物索引。
func (r *ArtifactRepo) Create(ctx context.Context, row *models.Artifact) error {
	return r.db.WithContext(ctx).Create(row).Error
}

// UpsertAfterRun 在 Run 成功后插入或更新产物索引。
func (r *ArtifactRepo) UpsertAfterRun(ctx context.Context, row *models.Artifact) error {
	existing, err := r.GetByProjectAndPath(ctx, row.ProjectID, row.Path)
	if err == nil {
		existing.RunID = row.RunID
		existing.PlanPath = row.PlanPath
		existing.Title = row.Title
		if row.RepositoryID != nil {
			existing.RepositoryID = row.RepositoryID
		}
		return r.Save(ctx, existing)
	}
	if !IsNotFound(err) {
		return err
	}
	return r.Create(ctx, row)
}
