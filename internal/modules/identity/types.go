package identity

import (
	"time"

	"github.com/google/uuid"
)

// User 是对外暴露的用户 DTO。
type User struct {
	ID           uuid.UUID  `json:"id"`
	Username     string     `json:"username"`
	Email        string     `json:"email"`
	Name         string     `json:"name"`
	AvatarURL    string     `json:"avatar_url,omitempty"`
	IsAdmin      bool       `json:"is_admin"`
	State        string     `json:"state"`
	LastSignInAt *time.Time `json:"last_sign_in_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
}

// CreateUserInput 是创建用户请求。
type CreateUserInput struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
	Name     string `json:"name"`
	IsAdmin  bool   `json:"is_admin"`
}

// UpdateUserInput 是更新用户请求（字段为 nil 表示不修改）。
type UpdateUserInput struct {
	Email    *string `json:"email"`
	Name     *string `json:"name"`
	IsAdmin  *bool   `json:"is_admin"`
	State    *string `json:"state"`
	Password *string `json:"password"`
}
