package webapp

import (
	"github.com/gin-gonic/gin"

	"matrix/internal/app"
	"matrix/internal/modules/systemsettings"
	"matrix/internal/platform/auth"
	platformhttp "matrix/internal/platform/http"
)

// registerSystemAdminRoutes 注册 root 专属的系统配置 API（/api/admin/system/settings/*）。
func registerSystemAdminRoutes(admin *gin.RouterGroup, d *app.Deps) {
	rootUser := d.Config.Auth.Bootstrap.AdminUsername
	sys := admin.Group("/system", auth.RequireRoot(rootUser))
	{
		ai := sys.Group("/settings/ai")
		ai.GET("", func(c *gin.Context) { getSystemAISettings(c, d) })
		ai.PUT("", func(c *gin.Context) { putSystemAISettings(c, d) })

		mcp := sys.Group("/settings/mcp")
		mcp.GET("", func(c *gin.Context) { getSystemMCPSettings(c, d) })
		mcp.PUT("", func(c *gin.Context) { putSystemMCPSettings(c, d) })

		git := sys.Group("/settings/git")
		git.GET("", func(c *gin.Context) { getSystemGitSettings(c, d) })
		git.PUT("", func(c *gin.Context) { putSystemGitSettings(c, d) })
		git.POST("/test", func(c *gin.Context) { postSystemGitTest(c, d) })

		worker := sys.Group("/settings/worker")
		worker.GET("", func(c *gin.Context) { getSystemWorkerSettings(c, d) })
		worker.PUT("", func(c *gin.Context) { putSystemWorkerSettings(c, d) })

		pipeline := sys.Group("/settings/pipeline")
		pipeline.GET("", func(c *gin.Context) { getSystemPipelineSettings(c, d) })
		pipeline.PUT("", func(c *gin.Context) { putSystemPipelineSettings(c, d) })
	}
}

func getSystemAISettings(c *gin.Context, d *app.Deps) {
	st, err := d.SystemSettings.GetAI(c.Request.Context())
	if err != nil {
		platformhttp.JSONError(c, 500, "internal", err.Error())
		return
	}
	c.JSON(200, st)
}

func putSystemAISettings(c *gin.Context, d *app.Deps) {
	var in systemsettings.AISettings
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

func getSystemMCPSettings(c *gin.Context, d *app.Deps) {
	st, err := d.SystemSettings.GetMCP(c.Request.Context())
	if err != nil {
		platformhttp.JSONError(c, 500, "internal", err.Error())
		return
	}
	c.JSON(200, st)
}

func putSystemMCPSettings(c *gin.Context, d *app.Deps) {
	var body struct {
		MCPServers map[string]systemsettings.MCPServerSettings `json:"mcp_servers"`
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

func getSystemGitSettings(c *gin.Context, d *app.Deps) {
	st, err := d.SystemSettings.GetGit(c.Request.Context())
	if err != nil {
		platformhttp.JSONError(c, 500, "internal", err.Error())
		return
	}
	c.JSON(200, st)
}

func putSystemGitSettings(c *gin.Context, d *app.Deps) {
	var in systemsettings.GitSettings
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

func getSystemWorkerSettings(c *gin.Context, d *app.Deps) {
	st, err := d.SystemSettings.GetWorker(c.Request.Context())
	if err != nil {
		platformhttp.JSONError(c, 500, "internal", err.Error())
		return
	}
	c.JSON(200, st)
}

func putSystemWorkerSettings(c *gin.Context, d *app.Deps) {
	var in systemsettings.WorkerSettings
	if c.BindJSON(&in) != nil {
		platformhttp.JSONError(c, 400, "bad_request", "无效请求")
		return
	}
	st, err := d.SystemSettings.SaveWorker(c.Request.Context(), in)
	if err != nil {
		platformhttp.JSONError(c, 400, "bad_request", err.Error())
		return
	}
	c.JSON(200, st)
}

func getSystemPipelineSettings(c *gin.Context, d *app.Deps) {
	st, err := d.SystemSettings.GetPipeline(c.Request.Context())
	if err != nil {
		platformhttp.JSONError(c, 500, "internal", err.Error())
		return
	}
	c.JSON(200, st)
}

func putSystemPipelineSettings(c *gin.Context, d *app.Deps) {
	var in systemsettings.PipelineSettings
	if c.BindJSON(&in) != nil {
		platformhttp.JSONError(c, 400, "bad_request", "无效请求")
		return
	}
	st, err := d.SystemSettings.SavePipeline(c.Request.Context(), in)
	if err != nil {
		platformhttp.JSONError(c, 400, "bad_request", err.Error())
		return
	}
	c.JSON(200, st)
}

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
