package repo

import (
	"context"

	"matrix/internal/platform/db/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// GroupMemberRepo 封装 GroupMember 表持久化操作。
type GroupMemberRepo struct {
	db *gorm.DB
}

// NewGroupMemberRepo 创建 GroupMemberRepo。
func NewGroupMemberRepo(db *gorm.DB) *GroupMemberRepo {
	return &GroupMemberRepo{db: db}
}

// Create 创建组成员。
func (r *GroupMemberRepo) Create(ctx context.Context, m *models.GroupMember) error {
	return r.db.WithContext(ctx).Create(m).Error
}

// Get 查询组成员。
func (r *GroupMemberRepo) Get(ctx context.Context, groupID, userID uuid.UUID) (*models.GroupMember, error) {
	var m models.GroupMember
	if err := r.db.WithContext(ctx).Where("group_id = ? AND user_id = ?", groupID, userID).First(&m).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

// ListByGroup 列出组成员。
func (r *GroupMemberRepo) ListByGroup(ctx context.Context, groupID uuid.UUID) ([]models.GroupMember, error) {
	var rows []models.GroupMember
	if err := r.db.WithContext(ctx).Where("group_id = ?", groupID).Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// Save 保存组成员。
func (r *GroupMemberRepo) Save(ctx context.Context, m *models.GroupMember) error {
	return r.db.WithContext(ctx).Save(m).Error
}

// UpdateRole 更新成员角色。
func (r *GroupMemberRepo) UpdateRole(ctx context.Context, groupID, userID uuid.UUID, role string) error {
	return r.db.WithContext(ctx).Model(&models.GroupMember{}).
		Where("group_id = ? AND user_id = ?", groupID, userID).
		Update("role", role).Error
}

// Delete 删除组成员。
func (r *GroupMemberRepo) Delete(ctx context.Context, groupID, userID uuid.UUID) error {
	return r.db.WithContext(ctx).
		Where("group_id = ? AND user_id = ?", groupID, userID).
		Delete(&models.GroupMember{}).Error
}
