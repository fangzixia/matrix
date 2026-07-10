// Package identity 用户账户、密码校验、Session 与首装 Bootstrap 管理员。
package identity

import (
	"context"
	"errors"
	"fmt"
	"matrix/internal/platform/config"
	"matrix/internal/platform/db/models"
	"matrix/internal/platform/db/repo"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// ErrInvalidCredentials 表示用户名或密码错误。
var ErrInvalidCredentials = errors.New("invalid credentials")

// ErrUserBlocked 表示用户已被封禁。
var ErrUserBlocked = errors.New("user blocked")

// UserRepo 提供用户持久化 CRUD，密码哈希在 identity 层处理。
type UserRepo struct {
	users *repo.UserStore
}

// NewUserRepo 创建 UserRepo。
func NewUserRepo(stores *repo.Stores) *UserRepo {
	return &UserRepo{users: stores.User}
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
	if err := r.users.Create(ctx, &m); err != nil {
		return nil, err
	}
	return toUser(&m), nil
}

// GetByID 按 ID 查询用户。
func (r *UserRepo) GetByID(ctx context.Context, id uuid.UUID) (*User, error) {
	m, err := r.users.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return toUser(m), nil
}

// GetByUsername 按用户名查询用户。
func (r *UserRepo) GetByUsername(ctx context.Context, username string) (*models.User, error) {
	return r.users.GetByUsername(ctx, username)
}

// GetByLogin 按登录名（用户名或邮箱）查询用户。
func (r *UserRepo) GetByLogin(ctx context.Context, login string) (*models.User, error) {
	return r.users.GetByLogin(ctx, login)
}

// Search 按关键字搜索。
func (r *UserRepo) Search(ctx context.Context, q string, limit int) ([]User, error) {
	rows, err := r.users.Search(ctx, q, limit)
	if err != nil {
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
	rows, total, err := r.users.List(ctx, limit, offset)
	if err != nil {
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
	m, err := r.users.GetByID(ctx, id)
	if err != nil {
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
	if err := r.users.Save(ctx, m); err != nil {
		return nil, err
	}
	return toUser(m), nil
}

// Delete 删除记录。
func (r *UserRepo) Delete(ctx context.Context, id uuid.UUID) error {
	return r.users.Delete(ctx, id)
}

// ResetPassword 重置用户密码。
func (r *UserRepo) ResetPassword(ctx context.Context, id uuid.UUID, password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return r.users.UpdatePasswordHash(ctx, id, string(hash))
}

// Block 封禁用户账户。
func (r *UserRepo) Block(ctx context.Context, id uuid.UUID) error {
	return r.users.UpdateState(ctx, id, "blocked")
}

// Unblock 解除用户封禁。
func (r *UserRepo) Unblock(ctx context.Context, id uuid.UUID) error {
	return r.users.UpdateState(ctx, id, "active")
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
		n, _ := r.users.CountProjectMemberships(ctx, u.ID)
		out[i] = UserWithStats{User: u, ProjectCount: n}
	}
	return out, total, nil
}

// Count 返回数量统计。
func (r *UserRepo) Count(ctx context.Context) (int64, error) {
	return r.users.Count(ctx)
}

// UpdateLastSignIn 更新用户最近登录时间。
func (r *UserRepo) UpdateLastSignIn(ctx context.Context, id uuid.UUID, t time.Time) error {
	return r.users.UpdateLastSignIn(ctx, id, t)
}

// toUser 转换为 User DTO。
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

// Logout 注销当前 Session。
func (s *AuthService) Logout(ctx context.Context, token string) error {
	return s.sessions.Revoke(ctx, token)
}

// BootstrapAdmin 在空库时创建配置中的首装管理员账户。
func BootstrapAdmin(ctx context.Context, stores *repo.Stores, cfg config.AuthConfig) error {
	users := NewUserRepo(stores)
	n, err := users.Count(ctx)
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
	_, err = users.Create(ctx, CreateUserInput{
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
