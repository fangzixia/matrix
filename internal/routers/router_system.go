package routers

import (
	"matrix/internal/app"
	"matrix/internal/modules/settings"
	"matrix/internal/platform/auth"
	platformhttp "matrix/internal/platform/http"

	"github.com/gin-gonic/gin"
)

// registerSystemAdminRoutes 注册 root 专属的系统配置 API（/api/admin/system/settings/*）。
func registerSystemAdminRoutes(admin *gin.RouterGroup, d *app.Deps) {
	rootUser := d.Config.Auth.Bootstrap.AdminUsername
	sys := admin.Group("/system", auth.RequireRoot(rootUser))
	{
		ai := sys.Group("/settings/ai")
		// 读取系统 AI 配置
		ai.GET("", func(c *gin.Context) { getSystemAISettings(c, d) })
		// 保存系统 AI 配置
		ai.PUT("", func(c *gin.Context) { putSystemAISettings(c, d) })
		mcp := sys.Group("/settings/mcp")
		// 读取系统 MCP 配置
		mcp.GET("", func(c *gin.Context) { getSystemMCPSettings(c, d) })
		// 保存系统 MCP 配置
		mcp.PUT("", func(c *gin.Context) { putSystemMCPSettings(c, d) })
		git := sys.Group("/settings/git")
		// 读取系统 Git 配置
		git.GET("", func(c *gin.Context) { getSystemGitSettings(c, d) })
		// 保存系统 Git 配置
		git.PUT("", func(c *gin.Context) { putSystemGitSettings(c, d) })
		// 测试 Git 仓库连通性
		git.POST("/test", func(c *gin.Context) { postSystemGitTest(c, d) })
	}
}

// getSystemAISettings 读取系统 AI 配置。
func getSystemAISettings(c *gin.Context, d *app.Deps) {
	st, err := d.SystemSettings.GetAI(c.Request.Context())
	if err != nil {
		platformhttp.JSONError(c, 500, "internal", err.Error())
		return
	}
	c.JSON(200, st)
}

// putSystemAISettings 保存系统 AI 配置。
func putSystemAISettings(c *gin.Context, d *app.Deps) {
	var in settings.AISettings
	if c.BindJSON(&in) != nil {
		platformhttp.JSONError(c, 400, "bad_request", "无效请求")
		return
	}
	st, err := d.SystemSettings.SaveAI(c.Request.Context(), in)
	if err != nil {
		platformhttp.JSONError(c, 400, "bad_request", err.Error())
		return
	}
	c.JSON(200, st)
}

// getSystemMCPSettings 读取系统 MCP 配置。
func getSystemMCPSettings(c *gin.Context, d *app.Deps) {
	st, err := d.SystemSettings.GetMCP(c.Request.Context())
	if err != nil {
		platformhttp.JSONError(c, 500, "internal", err.Error())
		return
	}
	c.JSON(200, st)
}

// putSystemMCPSettings 保存系统 MCP 配置。
func putSystemMCPSettings(c *gin.Context, d *app.Deps) {
	var body struct {
		MCPServers map[string]settings.MCPServerSettings `json:"mcp_servers"`
	}
	if c.BindJSON(&body) != nil {
		platformhttp.JSONError(c, 400, "bad_request", "无效请求")
		return
	}
	st, err := d.SystemSettings.SaveMCP(c.Request.Context(), body.MCPServers)
	if err != nil {
		platformhttp.JSONError(c, 400, "bad_request", err.Error())
		return
	}
	c.JSON(200, st)
}

// getSystemGitSettings 读取系统 Git 配置。
func getSystemGitSettings(c *gin.Context, d *app.Deps) {
	st, err := d.SystemSettings.GetGit(c.Request.Context())
	if err != nil {
		platformhttp.JSONError(c, 500, "internal", err.Error())
		return
	}
	c.JSON(200, st)
}

// putSystemGitSettings 保存系统 Git 配置。
func putSystemGitSettings(c *gin.Context, d *app.Deps) {
	var in settings.GitSettings
	if c.BindJSON(&in) != nil {
		platformhttp.JSONError(c, 400, "bad_request", "无效请求")
		return
	}
	st, err := d.SystemSettings.SaveGit(c.Request.Context(), in)
	if err != nil {
		platformhttp.JSONError(c, 400, "bad_request", err.Error())
		return
	}
	c.JSON(200, st)
}

// postSystemGitTest 测试 Git 仓库连通性。
func postSystemGitTest(c *gin.Context, d *app.Deps) {
	var body struct {
		GitURL string `json:"git_url"`
	}
	if c.BindJSON(&body) != nil {
		platformhttp.JSONError(c, 400, "bad_request", "无效请求")
		return
	}
	msg, err := d.SystemSettings.TestGit(c.Request.Context(), body.GitURL)
	if err != nil {
		platformhttp.JSONError(c, 400, "bad_request", err.Error())
		return
	}
	c.JSON(200, gin.H{"ok": true, "message": msg})
}
