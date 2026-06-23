package routers

import (
	"matrix/internal/app"
	"matrix/internal/modules/iam"
	"matrix/internal/modules/project"
	"matrix/internal/platform/auth"
	platformhttp "matrix/internal/platform/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// registerProjectRoutes 注册项目与成员管理 API。
func registerProjectRoutes(api *gin.RouterGroup, d *app.Deps) {
	authz := auth.RequireAuth(d.Sessions)
	api.GET("/projects", authz, func(c *gin.Context) { listProjects(c, d) })
	api.POST("/projects", authz, func(c *gin.Context) { createProject(c, d) })
	api.GET("/projects/:id", authz, auth.RequireProject(d.IAM, iam.RoleGuest), func(c *gin.Context) { getProject(c, d) })
	api.PUT("/projects/:id", authz, auth.RequireProject(d.IAM, iam.RoleMaintainer), func(c *gin.Context) { updateProject(c, d) })
	api.DELETE("/projects/:id", authz, auth.RequireProject(d.IAM, iam.RoleOwner), func(c *gin.Context) { deleteProject(c, d) })
	api.GET("/projects/:id/members", authz, auth.RequireProject(d.IAM, iam.RoleGuest), func(c *gin.Context) { listMembers(c, d) })
	api.POST("/projects/:id/members", authz, auth.RequireProject(d.IAM, iam.RoleMaintainer), func(c *gin.Context) { addMember(c, d) })
	api.PUT("/projects/:id/members/:uid", authz, auth.RequireProject(d.IAM, iam.RoleMaintainer), func(c *gin.Context) { updateMember(c, d) })
	api.DELETE("/projects/:id/members/:uid", authz, auth.RequireProject(d.IAM, iam.RoleMaintainer), func(c *gin.Context) { removeMember(c, d) })
}

// listProjects 列出Projects。
func listProjects(c *gin.Context, d *app.Deps) {
	u, _ := auth.User(c)
	scope := c.DefaultQuery("scope", "yours")
	list, err := d.Projects.ListForUser(c.Request.Context(), u.ID, u.IsAdmin, scope)
	if err != nil {
		platformhttp.JSONError(c, 500, "internal", err.Error())
		return
	}
	c.JSON(200, gin.H{"projects": list})
}

// createProject 创建Project。
func createProject(c *gin.Context, d *app.Deps) {
	u, _ := auth.User(c)
	var in project.CreateInput
	if c.BindJSON(&in) != nil {
		platformhttp.JSONError(c, 400, "bad_request", "无效请求")
		return
	}
	p, err := d.Projects.Create(c.Request.Context(), u.ID, in)
	if err != nil {
		platformhttp.JSONError(c, 400, "bad_request", err.Error())
		return
	}
	_ = d.Repositories.SeedDefault(c.Request.Context(), p.ID, p.GitURL, p.GitBranch)
	_ = d.Workspace.EnsureClone(c.Request.Context(), p.ID, p.GitURL, p.GitBranch)
	c.JSON(201, p)
}

// getProject 获取Project。
func getProject(c *gin.Context, d *app.Deps) {
	pid := auth.ProjectID(c)
	u, _ := auth.User(c)
	p, err := d.Projects.GetForUser(c.Request.Context(), pid, u.ID, u.IsAdmin)
	if err != nil {
		platformhttp.JSONError(c, 404, "not_found", "项目不存在")
		return
	}
	c.JSON(200, p)
}

// updateProject 更新Project。
func updateProject(c *gin.Context, d *app.Deps) {
	pid := auth.ProjectID(c)
	var body struct {
		Name       *string    `json:"name"`
		Path       *string    `json:"path"`
		GitURL     *string    `json:"git_url"`
		GitBranch  *string    `json:"git_branch"`
		Visibility *string    `json:"visibility"`
		GroupID    *uuid.UUID `json:"group_id"`
	}
	_ = c.BindJSON(&body)
	p, err := d.Projects.Update(c.Request.Context(), pid, body.Name, body.Path, body.GitURL, body.GitBranch, body.Visibility, body.GroupID)
	if err != nil {
		platformhttp.JSONError(c, 400, "bad_request", err.Error())
		return
	}
	c.JSON(200, p)
}

// deleteProject 删除Project。
func deleteProject(c *gin.Context, d *app.Deps) {
	pid := auth.ProjectID(c)
	if err := d.Projects.Delete(c.Request.Context(), pid); err != nil {
		platformhttp.JSONError(c, 500, "internal", err.Error())
		return
	}
	c.JSON(200, gin.H{"ok": true})
}

// listMembers 列出Members。
func listMembers(c *gin.Context, d *app.Deps) {
	pid := auth.ProjectID(c)
	list, err := d.Members.List(c.Request.Context(), pid)
	if err != nil {
		platformhttp.JSONError(c, 500, "internal", err.Error())
		return
	}
	c.JSON(200, gin.H{"members": list})
}

// addMember 添加Member。
func addMember(c *gin.Context, d *app.Deps) {
	pid := auth.ProjectID(c)
	var body struct {
		UserID   *uuid.UUID `json:"user_id"`
		Username *string    `json:"username"`
		Role     iam.Role   `json:"role"`
	}
	if c.BindJSON(&body) != nil {
		platformhttp.JSONError(c, 400, "bad_request", "无效请求")
		return
	}
	uid := uuid.Nil
	if body.UserID != nil && *body.UserID != uuid.Nil {
		uid = *body.UserID
	} else if body.Username != nil && *body.Username != "" {
		u, err := d.Users.GetByUsername(c.Request.Context(), *body.Username)
		if err != nil {
			platformhttp.JSONError(c, 404, "not_found", "用户不存在")
			return
		}
		uid = u.ID
	} else {
		platformhttp.JSONError(c, 400, "bad_request", "user_id 或 username 必填")
		return
	}
	if err := d.Members.Add(c.Request.Context(), pid, uid, body.Role); err != nil {
		platformhttp.JSONError(c, 400, "bad_request", err.Error())
		return
	}
	c.JSON(201, gin.H{"ok": true})
}

// updateMember 更新Member。
func updateMember(c *gin.Context, d *app.Deps) {
	pid := auth.ProjectID(c)
	uid, _ := uuid.Parse(c.Param("uid"))
	var body struct {
		Role iam.Role `json:"role"`
	}
	_ = c.BindJSON(&body)
	if err := d.Members.UpdateRole(c.Request.Context(), pid, uid, body.Role); err != nil {
		platformhttp.JSONError(c, 400, "bad_request", err.Error())
		return
	}
	c.JSON(200, gin.H{"ok": true})
}

// removeMember 移除Member。
func removeMember(c *gin.Context, d *app.Deps) {
	pid := auth.ProjectID(c)
	uid, _ := uuid.Parse(c.Param("uid"))
	_ = d.Members.Remove(c.Request.Context(), pid, uid)
	c.JSON(200, gin.H{"ok": true})
}
