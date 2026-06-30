package routers

import (
	"matrix/internal/app"
	"matrix/internal/modules/iam"
	"matrix/internal/modules/plan"
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
	api.GET("/projects/:id/repository/tree", authz, guest, func(c *gin.Context) { listRunRepoFiles(c, d) })
	api.GET("/projects/:id/repository/file", authz, guest, func(c *gin.Context) { readRunRepoFile(c, d) })
	api.GET("/projects/:id/plans", authz, guest, func(c *gin.Context) { listPlans(c, d) })
	api.POST("/projects/:id/plans/approve", authz, dev, func(c *gin.Context) { approvePlan(c, d) })
	api.GET("/projects/:id/evaluations", authz, guest, func(c *gin.Context) { listEvaluations(c, d) })
}

func parseRunIDQuery(c *gin.Context) (uuid.UUID, bool) {
	raw := c.Query("run_id")
	if raw == "" {
		platformhttp.JSONError(c, 400, "bad_request", "缺少 run_id")
		return uuid.Nil, false
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		platformhttp.JSONError(c, 400, "bad_request", "无效的 run_id")
		return uuid.Nil, false
	}
	return id, true
}

// listRunRepoFiles 列出 Run 仓库目录树。
func listRunRepoFiles(c *gin.Context, d *app.Deps) {
	pid := auth.ProjectID(c)
	runID, ok := parseRunIDQuery(c)
	if !ok {
		return
	}
	if _, err := d.Runs.GetForProject(c.Request.Context(), pid, runID); err != nil {
		platformhttp.JSONError(c, 404, "not_found", "运行不存在")
		return
	}
	path := c.Query("path")
	files, err := d.WorkspaceRepo.ListRunFiles(c.Request.Context(), pid, runID, path)
	if err != nil {
		platformhttp.JSONError(c, 500, "internal", err.Error())
		return
	}
	c.JSON(200, gin.H{"files": files})
}

// readRunRepoFile 读取 Run 仓库文件内容。
func readRunRepoFile(c *gin.Context, d *app.Deps) {
	pid := auth.ProjectID(c)
	runID, ok := parseRunIDQuery(c)
	if !ok {
		return
	}
	if _, err := d.Runs.GetForProject(c.Request.Context(), pid, runID); err != nil {
		platformhttp.JSONError(c, 404, "not_found", "运行不存在")
		return
	}
	content, err := d.WorkspaceRepo.ReadRunFile(c.Request.Context(), pid, runID, c.Query("path"))
	if err != nil {
		platformhttp.JSONError(c, 500, "internal", err.Error())
		return
	}
	c.JSON(200, gin.H{"content": content})
}

// listPlans 列出Plans。
func listPlans(c *gin.Context, d *app.Deps) {
	pid := auth.ProjectID(c)
	repoID, ok := parseRepositoryIDQuery(c)
	if !ok {
		return
	}
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
	repoID, ok := parseRepositoryIDQuery(c)
	if !ok {
		return
	}
	items, err := d.Artifacts.ListEvaluations(c.Request.Context(), pid, repoID)
	if err != nil {
		platformhttp.JSONError(c, 500, "internal", err.Error())
		return
	}
	c.JSON(200, gin.H{"evaluations": items})
}

// parseRepositoryIDQuery 从查询参数解析 repository_id。
func parseRepositoryIDQuery(c *gin.Context) (*uuid.UUID, bool) {
	raw := c.Query("repository_id")
	if raw == "" {
		return nil, true
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		platformhttp.JSONError(c, 400, "bad_request", "无效的仓库 ID")
		return nil, false
	}
	return &id, true
}
