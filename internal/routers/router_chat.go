package routers

import (
	"context"
	"strings"

	ai "matrix/ai/sdk"
	"matrix/internal/app"
	"matrix/internal/modules/iam"
	"matrix/internal/modules/run"
	"matrix/internal/platform/auth"
	"matrix/internal/platform/config"
	platformhttp "matrix/internal/platform/http"
	"matrix/internal/platform/logging"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// registerChatRoutes 注册 Chat 会话 API。
func registerChatRoutes(api *gin.RouterGroup, d *app.Deps) {
	authz := auth.RequireAuth(d.Sessions)
	guest := auth.RequireProject(d.IAM, iam.RoleGuest)
	dev := auth.RequireProject(d.IAM, iam.RoleDeveloper)
	// 列出 Chat 会话
	api.GET("/projects/:id/chat/sessions", authz, guest, func(c *gin.Context) { listChat(c, d) })
	// 创建 Chat 会话
	api.POST("/projects/:id/chat/sessions", authz, dev, func(c *gin.Context) { createChat(c, d) })
	// 获取 Chat 会话详情（含消息树）
	api.GET("/projects/:id/chat/sessions/:sid", authz, guest, func(c *gin.Context) { getChat(c, d) })
	// 更新会话标题
	api.PATCH("/projects/:id/chat/sessions/:sid", authz, dev, func(c *gin.Context) { patchChat(c, d) })
	// 获取可用 AI 模型与附件能力
	api.GET("/projects/:id/chat/capabilities", authz, guest, func(c *gin.Context) { chatCapabilities(c, d) })
	// 删除 Chat 会话
	api.DELETE("/projects/:id/chat/sessions/:sid", authz, dev, func(c *gin.Context) { deleteChat(c, d) })
	// 发送消息并触发 AI 回复
	api.POST("/projects/:id/chat/sessions/:sid/run", authz, dev, func(c *gin.Context) { runChat(c, d) })
}

type chatAttachmentDTO struct {
	Type     string `json:"type"`
	MimeType string `json:"mime_type"`
	Name     string `json:"name"`
	Data     string `json:"data"`
}

func modelDisplayName(p config.ModelProfile) string {
	name := p.Name
	if strings.TrimSpace(name) == "" {
		name = p.Model
	}
	return name
}

func attachmentTypesForProfile(p config.ModelProfile) []string {
	types := p.AttachmentTypes
	if !p.Multimodal {
		types = nil
	}
	if types == nil {
		return []string{}
	}
	return types
}

func listChat(c *gin.Context, d *app.Deps) {
	pid := auth.ProjectID(c)
	sessions, err := d.RunService.ListChatSessions(c.Request.Context(), pid)
	if err != nil {
		platformhttp.JSONError(c, 500, "internal", err.Error())
		return
	}
	c.JSON(200, gin.H{"sessions": sessions})
}

func createChat(c *gin.Context, d *app.Deps) {
	u, _ := auth.User(c)
	pid := auth.ProjectID(c)
	var body struct {
		ID    string `json:"id"`
		Title string `json:"title"`
	}
	if c.BindJSON(&body) != nil {
		platformhttp.JSONError(c, 400, "bad_request", "无效请求")
		return
	}
	var id uuid.UUID
	if body.ID != "" {
		parsed, err := uuid.Parse(body.ID)
		if err != nil {
			platformhttp.JSONError(c, 400, "bad_request", "无效会话 ID")
			return
		}
		id = parsed
	}
	session, err := d.RunService.CreateChatSession(c.Request.Context(), pid, u.ID, id, body.Title)
	if err != nil {
		platformhttp.JSONError(c, 500, "internal_error", err.Error())
		return
	}
	c.JSON(201, session)
}

func getChat(c *gin.Context, d *app.Deps) {
	pid := auth.ProjectID(c)
	sidParam := c.Param("sid")
	sessionID, err := uuid.Parse(sidParam)
	if err != nil {
		platformhttp.JSONError(c, 400, "bad_request", "无效会话 ID")
		return
	}
	session, err := d.RunService.GetChatSession(c.Request.Context(), pid, sessionID)
	if err != nil {
		platformhttp.JSONError(c, 404, "not_found", "会话不存在")
		return
	}
	c.JSON(200, session)
}

func patchChat(c *gin.Context, d *app.Deps) {
	pid := auth.ProjectID(c)
	sidParam := c.Param("sid")
	sessionID, err := uuid.Parse(sidParam)
	if err != nil {
		platformhttp.JSONError(c, 400, "bad_request", "无效会话 ID")
		return
	}
	var body struct {
		Title string `json:"title"`
	}
	if c.BindJSON(&body) != nil {
		platformhttp.JSONError(c, 400, "bad_request", "无效请求")
		return
	}
	session, err := d.RunService.UpdateChatSession(c.Request.Context(), pid, sessionID, body.Title)
	if err != nil {
		platformhttp.JSONError(c, 404, "not_found", err.Error())
		return
	}
	c.JSON(200, session)
}

func chatCapabilities(c *gin.Context, d *app.Deps) {
	aiCfg, err := d.SystemSettings.LoadAIConfig(c.Request.Context())
	if err != nil {
		platformhttp.JSONError(c, 500, "internal_error", "加载模型配置失败")
		return
	}
	profile, ok := aiCfg.ActiveModelProfile()
	modelName := ""
	multimodal := false
	attachmentTypes := []string{}
	if ok {
		modelName = modelDisplayName(profile)
		multimodal = profile.Multimodal
		attachmentTypes = attachmentTypesForProfile(profile)
	}
	c.JSON(200, gin.H{
		"model_name":       modelName,
		"multimodal":       multimodal,
		"attachment_types": attachmentTypes,
	})
}

func deleteChat(c *gin.Context, d *app.Deps) {
	pid := auth.ProjectID(c)
	sidParam := c.Param("sid")
	sessionID, err := uuid.Parse(sidParam)
	if err != nil {
		platformhttp.JSONError(c, 400, "bad_request", "无效会话 ID")
		return
	}
	if err := d.RunService.DeleteChatSession(c.Request.Context(), pid, sessionID); err != nil {
		platformhttp.JSONError(c, 404, "not_found", err.Error())
		return
	}
	c.JSON(200, gin.H{"ok": true})
}

func activeChatModel(ctx context.Context, d *app.Deps) (string, config.ModelProfile, error) {
	aiCfg, err := d.SystemSettings.LoadAIConfig(ctx)
	if err != nil {
		return "", config.ModelProfile{}, err
	}
	_, profile, err := aiCfg.ResolveModel("")
	if err != nil {
		return "", config.ModelProfile{}, err
	}
	return profile.ID, profile, nil
}

func runChat(c *gin.Context, d *app.Deps) {
	// 用户信息
	u, _ := auth.User(c)
	// project id
	pid := auth.ProjectID(c)
	// session id
	sidParam := c.Param("sid")
	var sessionID uuid.UUID
	if sidParam != "" && sidParam != "default" {
		if parsed, err := uuid.Parse(sidParam); err == nil {
			sessionID = parsed
		}
	}
	var body struct {
		// 消息文本
		Message string `json:"message"`
		// 上次消息 id
		ParentID *string `json:"parent_id"`
		// 附件信息
		Attachments []chatAttachmentDTO `json:"attachments,omitempty"`
	}
	if c.BindJSON(&body) != nil {
		platformhttp.JSONError(c, 400, "bad_request", "无效请求")
		return
	}
	if sessionID == uuid.Nil {
		platformhttp.JSONError(c, 400, "bad_request", "无效会话 ID")
		return
	}
	if _, err := d.RunService.GetChatSession(c.Request.Context(), pid, sessionID); err != nil {
		platformhttp.JSONError(c, 404, "not_found", "会话不存在")
		return
	}
	parentID := ""
	if body.ParentID != nil {
		parentID = strings.TrimSpace(*body.ParentID)
	}
	if err := d.RunService.ValidateParentDB(c.Request.Context(), sessionID, parentID); err != nil {
		platformhttp.JSONError(c, 400, "bad_request", err.Error())
		return
	}
	// 使用系统配置的默认模型
	modelID, profile, err := activeChatModel(c.Request.Context(), d)
	if err != nil {
		platformhttp.JSONError(c, 400, "bad_request", err.Error())
		return
	}
	// 检查附件
	if len(body.Attachments) > 0 {
		if !profile.Multimodal {
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
	// 格式化附件
	var attachments []ai.MessageAttachment
	for _, att := range body.Attachments {
		attachments = append(attachments, ai.MessageAttachment{
			Type: att.Type, MimeType: att.MimeType, Name: att.Name, Data: att.Data,
		})
	}
	// 保存用户消息
	userMessageID := uuid.New()
	if err := d.RunService.InsertChatUserMessage(c.Request.Context(), sessionID, parentID, body.Message, attachments, userMessageID); err != nil {
		platformhttp.JSONError(c, 500, "internal_error", "用户消息保存失败")
		return
	}
	// 提交任务
	rn, err := d.RunService.Start(c.Request.Context(), pid, u.ID, run.StartInput{
		Kind: run.KindChat, Title: body.Message, Message: body.Message,
		ModelID:       modelID,
		ChatSessionID: &sessionID, ChatUserMessageID: &userMessageID,
	})
	if err != nil {
		platformhttp.JSONError(c, 400, "bad_request", err.Error())
		return
	}
	rn.UserMessageID = userMessageID.String()
	logging.Agent("chat: Run 已创建",
		"run_id", rn.ID,
		"session_id", sessionID,
		"user_message_id", userMessageID,
		"status", rn.Status,
		"model_id", modelID,
	)
	// 返回 202 Accepted（已接受）
	c.JSON(202, rn)
}
