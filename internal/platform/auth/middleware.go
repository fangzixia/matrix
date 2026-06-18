// Package auth 提供 Gin 中间件：Session 鉴权、Admin/Root 与项目 RBAC。
package auth

import (
	"github.com/gin-gonic/gin"

	"matrix/internal/modules/identity"
	platformhttp "matrix/internal/platform/http"
)

const ctxUserKey = "auth_user"

// SetUser 将已认证用户写入 Gin 上下文。
func SetUser(c *gin.Context, u *identity.User) {
	c.Set(ctxUserKey, u)
}

// User 从 Gin 上下文读取已认证用户。
func User(c *gin.Context) (*identity.User, bool) {
	v, ok := c.Get(ctxUserKey)
	if !ok {
		return nil, false
	}
	u, ok := v.(*identity.User)
	return u, ok
}

// RequireAuth 校验 Session Cookie 并将用户写入上下文。
func RequireAuth(session *identity.SessionService) gin.HandlerFunc {
	return func(c *gin.Context) {
		token, err := c.Cookie(session.CookieName())
		if err != nil || token == "" {
			platformhttp.JSONError(c, 401, "unauthorized", "请先登录")
			return
		}
		u, err := session.Validate(c.Request.Context(), token)
		if err != nil {
			platformhttp.JSONError(c, 401, "unauthorized", "会话无效或已过期")
			return
		}
		SetUser(c, u)
		c.Next()
	}
}

// RequireAdmin 要求当前用户具有管理员标志。
func RequireAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		u, ok := User(c)
		if !ok || !u.IsAdmin {
			platformhttp.JSONError(c, 403, "forbidden", "需要管理员权限")
			return
		}
		c.Next()
	}
}

// RequireRoot 仅允许 bootstrap 配置的 root 用户（默认用户名为 root）。
func RequireRoot(rootUsername string) gin.HandlerFunc {
	if rootUsername == "" {
		rootUsername = "root"
	}
	return func(c *gin.Context) {
		u, ok := User(c)
		if !ok || u.Username != rootUsername {
			platformhttp.JSONError(c, 403, "forbidden", "仅 root 用户可访问系统配置")
			return
		}
		c.Next()
	}
}
