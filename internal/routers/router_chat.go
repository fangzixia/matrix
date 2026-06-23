package routers

import (
	"matrix/internal/ai/query"
	"matrix/internal/app"
	"matrix/internal/modules/iam"
	"matrix/internal/modules/run"
	"matrix/internal/platform/auth"
	platformhttp "matrix/internal/platform/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// registerChatRoutes 注册 Chat 会话 API。
func registerChatRoutes(api *gin.RouterGroup, d *app.Deps) {
	authz := auth.RequireAuth(d.Sessions)
	guest := auth.RequireProject(d.IAM, iam.RoleGuest)
	dev := auth.RequireProject(d.IAM, iam.RoleDeveloper)
	api.GET("/projects/:id/chat/sessions", authz, guest, func(c *gin.Context) { listChat(c, d) })
	api.PUT("/projects/:id/chat/sessions", authz, dev, func(c *gin.Context) { saveChat(c, d) })
	api.POST("/projects/:id/chat/sessions/:sid/run", authz, dev, func(c *gin.Context) { runChat(c, d) })
}

// listChat 列出Chat。
func listChat(c *gin.Context, d *app.Deps) {
	pid := auth.ProjectID(c)
	sessions, _ := d.Runs.ListChatSessions(c.Request.Context(), pid)
	c.JSON(200, gin.H{"sessions": sessions})
}

// saveChat 保存Chat。
func saveChat(c *gin.Context, d *app.Deps) {
	u, _ := auth.User(c)
	pid := auth.ProjectID(c)
	var body struct {
		Sessions []run.ChatSessionDTO `json:"sessions"`
	}
	_ = c.BindJSON(&body)
	_ = d.Runs.SaveChatSessions(c.Request.Context(), pid, u.ID, body.Sessions)
	c.JSON(200, gin.H{"ok": true})
}

// runChat 在项目上下文中执行一次对话 Run。
func runChat(c *gin.Context, d *app.Deps) {
	u, _ := auth.User(c)
	pid := auth.ProjectID(c)
	sidParam := c.Param("sid")
	var sessionID uuid.UUID
	if sidParam != "" && sidParam != "default" {
		if parsed, err := uuid.Parse(sidParam); err == nil {
			sessionID = parsed
		}
	}
	var body struct {
		Message string `json:"message"`
	}
	if c.BindJSON(&body) != nil {
		platformhttp.JSONError(c, 400, "bad_request", "无效请求")
		return
	}
	var history []query.Message
	if sessionID != uuid.Nil {
		sessions, _ := d.Runs.ListChatSessions(c.Request.Context(), pid)
		for _, s := range sessions {
			if s.ID == sessionID {
				history = run.MessagesFromJSON(s.Messages)
				break
			}
		}
	}
	rn, err := d.Runs.RunChat(c.Request.Context(), pid, u.ID, sessionID, body.Message, history)
	if err != nil {
		platformhttp.JSONError(c, 400, "bad_request", err.Error())
		return
	}
	c.JSON(202, rn)
}
