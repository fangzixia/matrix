package webapp

import (
	"github.com/gin-gonic/gin"

	"matrix/internal/app"
	"matrix/internal/modules/systemsettings"
	"matrix/internal/platform/auth"
	platformhttp "matrix/internal/platform/http"
)

func registerSystemAdminRoutes(admin *gin.RouterGroup, d *app.Deps) {
	rootUser := d.Config.Auth.Bootstrap.AdminUsername
	sys := admin.Group("/system", auth.RequireRoot(rootUser))
	{
		sys.GET("/settings", func(c *gin.Context) { getSystemSettings(c, d) })
		sys.PUT("/settings", func(c *gin.Context) { putSystemSettings(c, d) })
		sys.PUT("/settings/mcp", func(c *gin.Context) { putSystemMCPSettings(c, d) })
		sys.POST("/git/test", func(c *gin.Context) { postSystemGitTest(c, d) })
	}
}

func getSystemSettings(c *gin.Context, d *app.Deps) {
	st, err := d.SystemSettings.Get(c.Request.Context())
	if err != nil {
		platformhttp.JSONError(c, 500, "internal", err.Error())
		return
	}
	c.JSON(200, st)
}

func putSystemSettings(c *gin.Context, d *app.Deps) {
	var in systemsettings.Settings
	if c.BindJSON(&in) != nil {
		platformhttp.JSONError(c, 400, "bad_request", "无效请求")
		return
	}
	st, err := d.SystemSettings.Save(c.Request.Context(), in)
	if err != nil {
		platformhttp.JSONError(c, 400, "bad_request", err.Error())
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
