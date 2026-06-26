package routers

import (
	"matrix/internal/app"
	"matrix/internal/modules/identity"
	"matrix/internal/platform/auth"
	platformhttp "matrix/internal/platform/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// registerAdminUserRoutes 注册 Admin 用户管理与系统配置 API。
func registerAdminUserRoutes(api *gin.RouterGroup, d *app.Deps) {
	admin := api.Group("/admin", auth.RequireAuth(d.Sessions), auth.RequireAdmin())
	{
		// 列出所有用户及统计信息
		admin.GET("/users", func(c *gin.Context) { adminListUsers(c, d) })
		// 创建新用户
		admin.POST("/users", func(c *gin.Context) { adminCreateUser(c, d) })
		// 获取指定用户详情
		admin.GET("/users/:uid", func(c *gin.Context) { adminGetUser(c, d) })
		// 更新指定用户信息
		admin.PUT("/users/:uid", func(c *gin.Context) { adminUpdateUser(c, d) })
		// 删除指定用户
		admin.DELETE("/users/:uid", func(c *gin.Context) { adminDeleteUser(c, d) })
		// 重置用户密码
		admin.POST("/users/:uid/reset_password", func(c *gin.Context) { adminResetPassword(c, d) })
		// 封禁用户
		admin.POST("/users/:uid/block", func(c *gin.Context) { adminBlockUser(c, d) })
		// 解封用户
		admin.POST("/users/:uid/unblock", func(c *gin.Context) { adminUnblockUser(c, d) })
		registerSystemAdminRoutes(admin, d)
	}
}

// adminListUsers 处理管理员 ListUsers 请求。
func adminListUsers(c *gin.Context, d *app.Deps) {
	list, total, err := d.Users.ListWithStats(c.Request.Context(), 100, 0)
	if err != nil {
		platformhttp.JSONError(c, 500, "internal", err.Error())
		return
	}
	c.JSON(200, gin.H{"users": list, "total": total})
}

// adminCreateUser 处理管理员 CreateUser 请求。
func adminCreateUser(c *gin.Context, d *app.Deps) {
	var in identity.CreateUserInput
	if c.BindJSON(&in) != nil {
		platformhttp.JSONError(c, 400, "bad_request", "无效请求")
		return
	}
	u, err := d.Users.Create(c.Request.Context(), in)
	if err != nil {
		platformhttp.JSONError(c, 400, "bad_request", err.Error())
		return
	}
	c.JSON(201, u)
}

// adminGetUser 处理管理员 GetUser 请求。
func adminGetUser(c *gin.Context, d *app.Deps) {
	id, err := uuid.Parse(c.Param("uid"))
	if err != nil {
		platformhttp.JSONError(c, 400, "bad_request", "无效 ID")
		return
	}
	u, err := d.Users.GetByID(c.Request.Context(), id)
	if err != nil {
		platformhttp.JSONError(c, 404, "not_found", "用户不存在")
		return
	}
	c.JSON(200, u)
}

// adminUpdateUser 处理管理员 UpdateUser 请求。
func adminUpdateUser(c *gin.Context, d *app.Deps) {
	id, _ := uuid.Parse(c.Param("uid"))
	var in identity.UpdateUserInput
	if c.BindJSON(&in) != nil {
		platformhttp.JSONError(c, 400, "bad_request", "无效请求")
		return
	}
	u, err := d.Users.Update(c.Request.Context(), id, in)
	if err != nil {
		platformhttp.JSONError(c, 400, "bad_request", err.Error())
		return
	}
	c.JSON(200, u)
}

// adminDeleteUser 处理管理员 DeleteUser 请求。
func adminDeleteUser(c *gin.Context, d *app.Deps) {
	id, _ := uuid.Parse(c.Param("uid"))
	if err := d.Users.Delete(c.Request.Context(), id); err != nil {
		platformhttp.JSONError(c, 500, "internal", err.Error())
		return
	}
	c.JSON(200, gin.H{"ok": true})
}

// adminResetPassword 处理管理员 ResetPassword 请求。
func adminResetPassword(c *gin.Context, d *app.Deps) {
	id, err := uuid.Parse(c.Param("uid"))
	if err != nil {
		platformhttp.JSONError(c, 400, "bad_request", "无效 ID")
		return
	}
	var body struct {
		Password string `json:"password"`
	}
	if c.BindJSON(&body) != nil || body.Password == "" {
		platformhttp.JSONError(c, 400, "bad_request", "password 必填")
		return
	}
	if err := d.Users.ResetPassword(c.Request.Context(), id, body.Password); err != nil {
		platformhttp.JSONError(c, 500, "internal", err.Error())
		return
	}
	c.JSON(200, gin.H{"ok": true})
}

// adminBlockUser 处理管理员 BlockUser 请求。
func adminBlockUser(c *gin.Context, d *app.Deps) {
	id, err := uuid.Parse(c.Param("uid"))
	if err != nil {
		platformhttp.JSONError(c, 400, "bad_request", "无效 ID")
		return
	}
	if err := d.Users.Block(c.Request.Context(), id); err != nil {
		platformhttp.JSONError(c, 500, "internal", err.Error())
		return
	}
	c.JSON(200, gin.H{"ok": true})
}

// adminUnblockUser 处理管理员 UnblockUser 请求。
func adminUnblockUser(c *gin.Context, d *app.Deps) {
	id, err := uuid.Parse(c.Param("uid"))
	if err != nil {
		platformhttp.JSONError(c, 400, "bad_request", "无效 ID")
		return
	}
	if err := d.Users.Unblock(c.Request.Context(), id); err != nil {
		platformhttp.JSONError(c, 500, "internal", err.Error())
		return
	}
	c.JSON(200, gin.H{"ok": true})
}
