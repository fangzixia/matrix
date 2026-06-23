package routers

import (
	"strings"

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
	api.GET("/projects/:id/chat/capabilities", authz, guest, func(c *gin.Context) { chatCapabilities(c, d) })
	api.PUT("/projects/:id/chat/sessions", authz, dev, func(c *gin.Context) { saveChat(c, d) })
	api.POST("/projects/:id/chat/sessions/:sid/run", authz, dev, func(c *gin.Context) { runChat(c, d) })
}

type chatAttachmentDTO struct {
	Type     string `json:"type"`
	MimeType string `json:"mime_type"`
	Name     string `json:"name"`
	Data     string `json:"data"`
}

// listChat 列出Chat。
func listChat(c *gin.Context, d *app.Deps) {
	pid := auth.ProjectID(c)
	sessions, _ := d.Runs.ListChatSessions(c.Request.Context(), pid)
	c.JSON(200, gin.H{"sessions": sessions})
}

// chatCapabilities 返回当前生效模型的多模态能力。
func chatCapabilities(c *gin.Context, d *app.Deps) {
	profile, ok := d.Runtime.AI.ActiveModelProfile()
	if !ok {
		c.JSON(200, gin.H{
			"model_name":       "",
			"multimodal":       false,
			"attachment_types": []string{},
		})
		return
	}
	name := profile.Name
	if strings.TrimSpace(name) == "" {
		name = profile.Model
	}
	types := profile.AttachmentTypes
	if !profile.Multimodal {
		types = nil
	}
	if types == nil {
		types = []string{}
	}
	c.JSON(200, gin.H{
		"model_name":       name,
		"multimodal":       profile.Multimodal,
		"attachment_types": types,
	})
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
		Message     string              `json:"message"`
		Attachments []chatAttachmentDTO `json:"attachments,omitempty"`
	}
	if c.BindJSON(&body) != nil {
		platformhttp.JSONError(c, 400, "bad_request", "无效请求")
		return
	}
	profile, modelOK := d.Runtime.AI.ActiveModelProfile()
	if len(body.Attachments) > 0 {
		if !modelOK || !profile.Multimodal {
			platformhttp.JSONError(c, 400, "bad_request", "当前模型不支持多模态附件")
			return
		}
		for _, att := range body.Attachments {
			if !profile.AllowsAttachmentType(att.Type) {
				platformhttp.JSONError(c, 400, "bad_request", "不支持的附件类型: "+att.Type)
				return
			}
		}
	}
	var attachments []query.MessageAttachment
	for _, att := range body.Attachments {
		attachments = append(attachments, query.MessageAttachment{
			Type: att.Type, MimeType: att.MimeType, Name: att.Name, Data: att.Data,
		})
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
	rn, err := d.Runs.RunChat(c.Request.Context(), pid, u.ID, sessionID, body.Message, attachments, history)
	if err != nil {
		platformhttp.JSONError(c, 400, "bad_request", err.Error())
		return
	}
	c.JSON(202, rn)
}
