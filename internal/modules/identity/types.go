package identity

import (
	"time"

	"github.com/google/uuid"
)

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

type CreateUserInput struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
	Name     string `json:"name"`
	IsAdmin  bool   `json:"is_admin"`
}

type UpdateUserInput struct {
	Email    *string `json:"email"`
	Name     *string `json:"name"`
	IsAdmin  *bool   `json:"is_admin"`
	State    *string `json:"state"`
	Password *string `json:"password"`
}
