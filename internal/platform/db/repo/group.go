package repo

import (
	"context"

	"matrix/internal/platform/db/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// GroupRepo 封装 Group 表持久化操作。
type GroupRepo struct {
	db *gorm.DB
}

// NewGroupRepo 创建 GroupRepo。
func NewGroupRepo(db *gorm.DB) *GroupRepo {
	return &GroupRepo{db: db}
}

// Create 创建用户组。
func (r *GroupRepo) Create(ctx context.Context, m *models.Group) error {
	return r.db.WithContext(ctx).Create(m).Error
}

// CreateWithOwner 原子创建用户组并添加 owner 成员。
func (r *GroupRepo) CreateWithOwner(ctx context.Context, m *models.Group, ownerID uuid.UUID, ownerRole string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.WithContext(ctx).Create(m).Error; err != nil {
			return err
		}
		return tx.WithContext(ctx).Create(&models.GroupMember{
			GroupID: m.ID, UserID: ownerID, Role: ownerRole,
		}).Error
	})
}

// GetByID 按 ID 查询用户组。
func (r *GroupRepo) GetByID(ctx context.Context, id uuid.UUID) (*models.Group, error) {
	var m models.Group
	if err := r.db.WithContext(ctx).First(&m, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

// Save 保存用户组。
func (r *GroupRepo) Save(ctx context.Context, m *models.Group) error {
	return r.db.WithContext(ctx).Save(m).Error
}

// Delete 删除用户组（不含级联，由业务层编排）。
func (r *GroupRepo) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&models.Group{}, "id = ?", id).Error
}

// ListForUser 列出用户可见的用户组。
func (r *GroupRepo) ListForUser(ctx context.Context, userID uuid.UUID, isAdmin bool) ([]models.Group, error) {
	q := r.db.WithContext(ctx).Model(&models.Group{})
	if !isAdmin {
		q = q.Where(
			"owner_id = ? OR id IN (?)",
			userID,
			r.db.Model(&models.GroupMember{}).Select("group_id").Where("user_id = ?", userID),
		)
	}
	var rows []models.Group
	if err := q.Order("name asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// DeleteWithCleanup 删除用户组并清理关联数据。
func (r *GroupRepo) DeleteWithCleanup(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&models.Project{}).Where("group_id = ?", id).Update("group_id", nil).Error; err != nil {
			return err
		}
		if err := tx.Where("group_id = ?", id).Delete(&models.GroupMember{}).Error; err != nil {
			return err
		}
		return tx.Delete(&models.Group{}, "id = ?", id).Error
	})
}
