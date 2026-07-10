package repo

import (
	"context"
	"strings"
	"time"

	"matrix/internal/platform/db/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// UserRepo 封装 User 表持久化操作。
type UserRepo struct {
	db *gorm.DB
}

// NewUserRepo 创建 UserRepo。
func NewUserRepo(db *gorm.DB) *UserRepo {
	return &UserRepo{db: db}
}

// Create 创建用户。
func (r *UserRepo) Create(ctx context.Context, m *models.User) error {
	return r.db.WithContext(ctx).Create(m).Error
}

// GetByID 按 ID 查询用户。
func (r *UserRepo) GetByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	var m models.User
	if err := r.db.WithContext(ctx).First(&m, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

// GetByUsername 按用户名查询。
func (r *UserRepo) GetByUsername(ctx context.Context, username string) (*models.User, error) {
	var m models.User
	if err := r.db.WithContext(ctx).Where("username = ?", username).First(&m).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

// GetByLogin 按登录名（用户名或邮箱）查询。
func (r *UserRepo) GetByLogin(ctx context.Context, login string) (*models.User, error) {
	var m models.User
	if err := r.db.WithContext(ctx).
		Where("username = ? OR email = ?", login, login).
		First(&m).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

// Search 按关键字搜索活跃用户。
func (r *UserRepo) Search(ctx context.Context, q string, limit int) ([]models.User, error) {
	if limit <= 0 {
		limit = 20
	}
	q = strings.TrimSpace(q)
	if q == "" {
		return []models.User{}, nil
	}
	like := "%" + q + "%"
	var rows []models.User
	if err := r.db.WithContext(ctx).
		Where("state = ? AND (username ILIKE ? OR name ILIKE ? OR email ILIKE ?)", "active", like, like, like).
		Order("username asc").
		Limit(limit).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// List 分页列出用户。
func (r *UserRepo) List(ctx context.Context, limit, offset int) ([]models.User, int64, error) {
	var rows []models.User
	var total int64
	q := r.db.WithContext(ctx).Model(&models.User{})
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if limit <= 0 {
		limit = 50
	}
	if err := q.Order("created_at desc").Limit(limit).Offset(offset).Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

// Save 保存用户。
func (r *UserRepo) Save(ctx context.Context, m *models.User) error {
	return r.db.WithContext(ctx).Save(m).Error
}

// Delete 删除用户。
func (r *UserRepo) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&models.User{}, "id = ?", id).Error
}

// UpdatePasswordHash 更新密码哈希。
func (r *UserRepo) UpdatePasswordHash(ctx context.Context, id uuid.UUID, hash string) error {
	return r.db.WithContext(ctx).Model(&models.User{}).Where("id = ?", id).
		Update("password_hash", hash).Error
}

// UpdateState 更新账户状态。
func (r *UserRepo) UpdateState(ctx context.Context, id uuid.UUID, state string) error {
	return r.db.WithContext(ctx).Model(&models.User{}).Where("id = ?", id).
		Update("state", state).Error
}

// UpdateLastSignIn 更新最近登录时间。
func (r *UserRepo) UpdateLastSignIn(ctx context.Context, id uuid.UUID, t time.Time) error {
	return r.db.WithContext(ctx).Model(&models.User{}).Where("id = ?", id).
		Update("last_sign_in_at", t).Error
}

// Count 返回用户总数。
func (r *UserRepo) Count(ctx context.Context) (int64, error) {
	var n int64
	err := r.db.WithContext(ctx).Model(&models.User{}).Count(&n).Error
	return n, err
}

// CountProjectMemberships 统计用户参与的项目数。
func (r *UserRepo) CountProjectMemberships(ctx context.Context, userID uuid.UUID) (int64, error) {
	var n int64
	err := r.db.WithContext(ctx).Model(&models.ProjectMember{}).
		Where("user_id = ?", userID).Count(&n).Error
	return n, err
}
