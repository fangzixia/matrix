package repo

import (
	"context"
	"strings"

	"matrix/internal/platform/db/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ProjectRepo 封装 Project 表持久化操作。
type ProjectRepo struct {
	db *gorm.DB
}

// NewProjectRepo 创建 ProjectRepo。
func NewProjectRepo(db *gorm.DB) *ProjectRepo {
	return &ProjectRepo{db: db}
}

// Create 创建项目。
func (r *ProjectRepo) Create(ctx context.Context, m *models.Project) error {
	return r.db.WithContext(ctx).Create(m).Error
}

// CreateWithOwner 原子创建项目并添加 owner 成员。
func (r *ProjectRepo) CreateWithOwner(ctx context.Context, m *models.Project, ownerID uuid.UUID, ownerRole string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.WithContext(ctx).Create(m).Error; err != nil {
			return err
		}
		return tx.WithContext(ctx).Create(&models.ProjectMember{
			ProjectID: m.ID, UserID: ownerID, Role: ownerRole,
		}).Error
	})
}

// GetByID 按 ID 查询项目。
func (r *ProjectRepo) GetByID(ctx context.Context, id uuid.UUID) (*models.Project, error) {
	var m models.Project
	if err := r.db.WithContext(ctx).First(&m, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

// GetGroupAndOwner 查询项目的 group_id 与 owner_id。
func (r *ProjectRepo) GetGroupAndOwner(ctx context.Context, projectID uuid.UUID) (groupID *uuid.UUID, ownerID uuid.UUID, err error) {
	var p models.Project
	if err := r.db.WithContext(ctx).Select("group_id", "owner_id").First(&p, "id = ?", projectID).Error; err != nil {
		return nil, uuid.Nil, err
	}
	return p.GroupID, p.OwnerID, nil
}

// GetVisibility 查询项目可见性。
func (r *ProjectRepo) GetVisibility(ctx context.Context, projectID uuid.UUID) (string, error) {
	var p models.Project
	if err := r.db.WithContext(ctx).Select("visibility").First(&p, "id = ?", projectID).Error; err != nil {
		return "", err
	}
	if p.Visibility == "" {
		return "private", nil
	}
	return p.Visibility, nil
}

// ExistsByPath 检查项目编码是否已被占用。
func (r *ProjectRepo) ExistsByPath(ctx context.Context, path string, excludeID uuid.UUID) (bool, error) {
	q := r.db.WithContext(ctx).Model(&models.Project{}).Where("path = ?", path)
	if excludeID != uuid.Nil {
		q = q.Where("id <> ?", excludeID)
	}
	var count int64
	if err := q.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// Save 保存项目。
func (r *ProjectRepo) Save(ctx context.Context, m *models.Project) error {
	return r.db.WithContext(ctx).Save(m).Error
}

// Delete 删除项目。
func (r *ProjectRepo) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&models.Project{}, "id = ?", id).Error
}

// ListAll 列出全部项目（迁移等场景）。
func (r *ProjectRepo) ListAll(ctx context.Context) ([]models.Project, error) {
	var rows []models.Project
	if err := r.db.WithContext(ctx).Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// ListForUser 按可见性规则列出用户可见项目。
func (r *ProjectRepo) ListForUser(ctx context.Context, userID uuid.UUID, isAdmin bool, scope string, visInternal, visPublic string) ([]models.Project, error) {
	q := r.db.WithContext(ctx).Model(&models.Project{})
	switch scope {
	case "explore":
		if !isAdmin {
			q = q.Where("visibility IN ?", []string{visInternal, visPublic})
		}
	case "starred":
		return []models.Project{}, nil
	default:
		if strings.HasPrefix(scope, "group/") {
			gid, err := uuid.Parse(strings.TrimPrefix(scope, "group/"))
			if err != nil {
				return nil, err
			}
			q = q.Where("group_id = ?", gid)
			if !isAdmin {
				q = q.Where(
					"owner_id = ? OR id IN (?) OR group_id IN (?)",
					userID,
					r.db.Model(&models.ProjectMember{}).Select("project_id").Where("user_id = ?", userID),
					r.db.Model(&models.GroupMember{}).Select("group_id").Where("user_id = ?", userID),
				)
			}
			break
		}
		if !isAdmin {
			q = q.Where(
				"owner_id = ? OR id IN (?) OR group_id IN (?) OR visibility IN ?",
				userID,
				r.db.Model(&models.ProjectMember{}).Select("project_id").Where("user_id = ?", userID),
				r.db.Model(&models.GroupMember{}).Select("group_id").Where("user_id = ?", userID),
				[]string{visInternal, visPublic},
			)
		}
	}
	var rows []models.Project
	if err := q.Order("updated_at desc").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// ClearGroupID 清空指定组的 group_id 引用。
func (r *ProjectRepo) ClearGroupID(ctx context.Context, groupID uuid.UUID) error {
	return r.db.WithContext(ctx).Model(&models.Project{}).Where("group_id = ?", groupID).Update("group_id", nil).Error
}
