package routers

import (
	"matrix/internal/app"
	"matrix/internal/modules/iam"
	"matrix/internal/modules/plan"
	"matrix/internal/modules/workspace"
	"matrix/internal/platform/auth"
	platformhttp "matrix/internal/platform/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// registerWorkspaceRoutes 注册 Git 工作区、计划与评估 API。
func registerWorkspaceRoutes(api *gin.RouterGroup, d *app.Deps) {
	authz := auth.RequireAuth(d.Sessions)
	guest := auth.RequireProject(d.IAM, iam.RoleGuest)
	dev := auth.RequireProject(d.IAM, iam.RoleDeveloper)
	maint := auth.RequireProject(d.IAM, iam.RoleMaintainer)
	// 列出仓库目录树
	api.GET("/projects/:id/repository/tree", authz, guest, func(c *gin.Context) { listFiles(c, d) })
	// 读取仓库文件内容
	api.GET("/projects/:id/repository/file", authz, guest, func(c *gin.Context) { readFile(c, d) })
	// 拉取默认仓库最新代码
	api.POST("/projects/:id/repository/pull", authz, dev, func(c *gin.Context) { pullRepo(c, d) })
	// 推送默认仓库本地提交
	api.POST("/projects/:id/repository/push", authz, maint, func(c *gin.Context) { pushRepo(c, d) })
	// 列出项目计划
	api.GET("/projects/:id/plans", authz, guest, func(c *gin.Context) { listPlans(c, d) })
	// 批准计划
	api.POST("/projects/:id/plans/approve", authz, dev, func(c *gin.Context) { approvePlan(c, d) })
	// 列出评估结果
	api.GET("/projects/:id/evaluations", authz, guest, func(c *gin.Context) { listEvaluations(c, d) })
}

// listFiles 列出Files。
func listFiles(c *gin.Context, d *app.Deps) {
	pid := auth.ProjectID(c)
	path := c.Query("path")
	repoName := repoNameForQuery(c, d, pid)
	var files []workspace.FileEntry
	var err error
	if repoName != "" {
		files, err = d.Workspace.ListFilesFor(pid, repoName, path)
	} else {
		files, err = d.Workspace.ListFiles(pid, path)
	}
	if err != nil {
		platformhttp.JSONError(c, 500, "internal", err.Error())
		return
	}
	c.JSON(200, gin.H{"files": files})
}

// readFile 读取File。
func readFile(c *gin.Context, d *app.Deps) {
	pid := auth.ProjectID(c)
	repoName := repoNameForQuery(c, d, pid)
	var content string
	var err error
	if repoName != "" {
		content, err = d.Workspace.ReadFileFor(pid, repoName, c.Query("path"))
	} else {
		content, err = d.Workspace.ReadFile(pid, c.Query("path"))
	}
	if err != nil {
		platformhttp.JSONError(c, 500, "internal", err.Error())
		return
	}
	c.JSON(200, gin.H{"content": content})
}

// listPlans 列出Plans。
func listPlans(c *gin.Context, d *app.Deps) {
	pid := auth.ProjectID(c)
	repoID := parseRepositoryIDQuery(c)
	items, err := d.Plans.List(c.Request.Context(), pid, repoID)
	if err != nil {
		platformhttp.JSONError(c, 500, "internal", err.Error())
		return
	}
	c.JSON(200, gin.H{"plans": items})
}

// approvePlan 批准Plan。
func approvePlan(c *gin.Context, d *app.Deps) {
	pid := auth.ProjectID(c)
	var in plan.ConfirmInput
	if c.BindJSON(&in) != nil {
		platformhttp.JSONError(c, 400, "bad_request", "无效请求")
		return
	}
	if err := d.Plans.Approve(c.Request.Context(), pid, in); err != nil {
		platformhttp.JSONError(c, 400, "bad_request", err.Error())
		return
	}
	c.JSON(200, gin.H{"ok": true, "status": plan.StatusApproved})
}

// listEvaluations 列出Evaluations。
func listEvaluations(c *gin.Context, d *app.Deps) {
	pid := auth.ProjectID(c)
	repoID := parseRepositoryIDQuery(c)
	items, err := d.Artifacts.ListEvaluations(c.Request.Context(), pid, repoID)
	if err != nil {
		platformhttp.JSONError(c, 500, "internal", err.Error())
		return
	}
	c.JSON(200, gin.H{"evaluations": items})
}

// parseRepositoryIDQuery 从查询参数解析 repository_id。
func parseRepositoryIDQuery(c *gin.Context) *uuid.UUID {
	raw := c.Query("repository_id")
	if raw == "" {
		return nil
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		return nil
	}
	return &id
}

// pullRepo 拉取项目默认仓库最新代码。
func pullRepo(c *gin.Context, d *app.Deps) {
	pid := auth.ProjectID(c)
	if err := d.Workspace.Pull(c.Request.Context(), pid); err != nil {
		platformhttp.JSONError(c, 500, "internal", err.Error())
		return
	}
	c.JSON(200, gin.H{"ok": true})
}

// pushRepo 推送项目默认仓库本地提交。
func pushRepo(c *gin.Context, d *app.Deps) {
	pid := auth.ProjectID(c)
	var body struct {
		Message string `json:"message"`
	}
	_ = c.BindJSON(&body)
	if err := d.Workspace.Push(c.Request.Context(), pid, body.Message); err != nil {
		platformhttp.JSONError(c, 500, "internal", err.Error())
		return
	}
	c.JSON(200, gin.H{"ok": true})
}
