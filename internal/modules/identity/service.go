// Package identity 用户账户、密码校验、Session 与首装 Bootstrap 管理员。
package identity

import (
	"context"
	"errors"
	"fmt"
	"matrix/internal/platform/config"
	"matrix/internal/platform/db/models"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// ErrInvalidCredentials 表示用户名或密码错误。
var ErrInvalidCredentials = errors.New("invalid credentials")

// ErrUserBlocked 表示用户已被封禁。
var ErrUserBlocked = errors.New("user blocked")

// UserRepo 提供用户持久化 CRUD。
type UserRepo struct {
	db *gorm.DB
}

// NewUserRepo 创建 UserRepo。
func NewUserRepo(db *gorm.DB) *UserRepo {
	return &UserRepo{db: db}
}

// Create 创建记录。
func (r *UserRepo) Create(ctx context.Context, in CreateUserInput) (*User, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	m := models.User{
		Username:     in.Username,
		Email:        in.Email,
		PasswordHash: string(hash),
		Name:         in.Name,
		IsAdmin:      in.IsAdmin,
		State:        "active",
	}
	if err := r.db.WithContext(ctx).Create(&m).Error; err != nil {
		return nil, err
	}
	return toUser(&m), nil
}

// GetByID 按 ID 查询用户。
func (r *UserRepo) GetByID(ctx context.Context, id uuid.UUID) (*User, error) {
	var m models.User
	if err := r.db.WithContext(ctx).First(&m, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return toUser(&m), nil
}

// GetByUsername 按用户名查询用户。
func (r *UserRepo) GetByUsername(ctx context.Context, username string) (*models.User, error) {
	var m models.User
	if err := r.db.WithContext(ctx).Where("username = ?", username).First(&m).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

// GetByLogin 按登录名（用户名或邮箱）查询用户。
func (r *UserRepo) GetByLogin(ctx context.Context, login string) (*models.User, error) {
	var m models.User
	if err := r.db.WithContext(ctx).
		Where("username = ? OR email = ?", login, login).
		First(&m).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

// Search 按关键字搜索。
func (r *UserRepo) Search(ctx context.Context, q string, limit int) ([]User, error) {
	if limit <= 0 {
		limit = 20
	}
	q = strings.TrimSpace(q)
	if q == "" {
		return []User{}, nil
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
	out := make([]User, len(rows))
	for i := range rows {
		out[i] = *toUser(&rows[i])
	}
	return out, nil
}

// List 返回列表。
func (r *UserRepo) List(ctx context.Context, limit, offset int) ([]User, int64, error) {
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
	out := make([]User, len(rows))
	for i := range rows {
		out[i] = *toUser(&rows[i])
	}
	return out, total, nil
}

// Update 更新记录。
func (r *UserRepo) Update(ctx context.Context, id uuid.UUID, in UpdateUserInput) (*User, error) {
	var m models.User
	if err := r.db.WithContext(ctx).First(&m, "id = ?", id).Error; err != nil {
		return nil, err
	}
	if in.Email != nil {
		m.Email = *in.Email
	}
	if in.Name != nil {
		m.Name = *in.Name
	}
	if in.IsAdmin != nil {
		m.IsAdmin = *in.IsAdmin
	}
	if in.State != nil {
		m.State = *in.State
	}
	if in.Password != nil && *in.Password != "" {
		hash, err := bcrypt.GenerateFromPassword([]byte(*in.Password), bcrypt.DefaultCost)
		if err != nil {
			return nil, err
		}
		m.PasswordHash = string(hash)
	}
	if err := r.db.WithContext(ctx).Save(&m).Error; err != nil {
		return nil, err
	}
	return toUser(&m), nil
}

// Delete 删除记录。
func (r *UserRepo) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&models.User{}, "id = ?", id).Error
}

// ResetPassword 重置用户密码。
func (r *UserRepo) ResetPassword(ctx context.Context, id uuid.UUID, password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return r.db.WithContext(ctx).Model(&models.User{}).Where("id = ?", id).
		Update("password_hash", string(hash)).Error
}

// Block 封禁用户账户。
func (r *UserRepo) Block(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Model(&models.User{}).Where("id = ?", id).
		Update("state", "blocked").Error
}

// Unblock 解除用户封禁。
func (r *UserRepo) Unblock(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Model(&models.User{}).Where("id = ?", id).
		Update("state", "active").Error
}

// UserWithStats 是带项目数量统计的用户 DTO。
type UserWithStats struct {
	User
	ProjectCount int64 `json:"project_count"`
}

// ListWithStats 返回带统计信息的用户列表。
func (r *UserRepo) ListWithStats(ctx context.Context, limit, offset int) ([]UserWithStats, int64, error) {
	users, total, err := r.List(ctx, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	out := make([]UserWithStats, len(users))
	for i, u := range users {
		var n int64
		_ = r.db.WithContext(ctx).Model(&models.ProjectMember{}).
			Where("user_id = ?", u.ID).Count(&n).Error
		out[i] = UserWithStats{User: u, ProjectCount: n}
	}
	return out, total, nil
}

// Count 返回数量统计。
func (r *UserRepo) Count(ctx context.Context) (int64, error) {
	var n int64
	err := r.db.WithContext(ctx).Model(&models.User{}).Count(&n).Error
	return n, err
}

// toUser 转换为User。
func toUser(m *models.User) *User {
	return &User{
		ID: m.ID, Username: m.Username, Email: m.Email, Name: m.Name,
		AvatarURL: m.AvatarURL, IsAdmin: m.IsAdmin, State: m.State,
		LastSignInAt: m.LastSignInAt, CreatedAt: m.CreatedAt,
	}
}

// AuthService 协调登录、登出与用户校验。
type AuthService struct {
	users    *UserRepo
	sessions *SessionService
}

// NewAuthService 创建 AuthService。
func NewAuthService(users *UserRepo, sessions *SessionService) *AuthService {
	return &AuthService{users: users, sessions: sessions}
}

// Login 校验凭据并创建 Session。
func (s *AuthService) Login(ctx context.Context, username, password, ip, ua string) (*User, string, error) {
	m, err := s.users.GetByLogin(ctx, username)
	if err != nil {
		return nil, "", ErrInvalidCredentials
	}
	if m.State == "blocked" {
		return nil, "", ErrUserBlocked
	}
	if err := bcrypt.CompareHashAndPassword([]byte(m.PasswordHash), []byte(password)); err != nil {
		return nil, "", ErrInvalidCredentials
	}
	token, err := s.sessions.Create(ctx, m.ID, ip, ua)
	if err != nil {
		return nil, "", err
	}
	now := time.Now()
	_ = s.users.UpdateLastSignIn(ctx, m.ID, now)
	return toUser(m), token, nil
}

// UpdateLastSignIn 更新用户最近登录时间。
func (r *UserRepo) UpdateLastSignIn(ctx context.Context, id uuid.UUID, t time.Time) error {
	return r.db.WithContext(ctx).Model(&models.User{}).Where("id = ?", id).Update("last_sign_in_at", t).Error
}

// Logout 注销当前 Session。
func (s *AuthService) Logout(ctx context.Context, token string) error {
	return s.sessions.Revoke(ctx, token)
}

// BootstrapAdmin 在空库时创建配置中的首装管理员账户。
func BootstrapAdmin(ctx context.Context, db *gorm.DB, cfg config.AuthConfig) error {
	repo := NewUserRepo(db)
	n, err := repo.Count(ctx)
	if err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	pass := cfg.Bootstrap.AdminPassword
	if pass == "" {
		pass = "changeme"
	}
	user := cfg.Bootstrap.AdminUsername
	if user == "" {
		user = "root"
	}
	_, err = repo.Create(ctx, CreateUserInput{
		Username: user,
		Email:    user + "@localhost",
		Password: pass,
		Name:     "Administrator",
		IsAdmin:  true,
	})
	if err != nil {
		return fmt.Errorf("bootstrap admin: %w", err)
	}
	return nil
}
