package routers

import (
	"fmt"
	"matrix/internal/app"
	"matrix/internal/modules/group"
	"matrix/internal/modules/iam"
	"matrix/internal/modules/repository"
	"matrix/internal/platform/auth"
	"matrix/internal/platform/events"
	platformhttp "matrix/internal/platform/http"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// registerMgmtRoutes 注册用户组、多仓库与 Run 详情等管理 API。
func registerMgmtRoutes(api *gin.RouterGroup, d *app.Deps) {
	// 列出当前用户可见的用户组
	api.GET("/groups", auth.RequireAuth(d.Sessions), func(c *gin.Context) { listGroups(c, d) })
	// 创建用户组
	api.POST("/groups", auth.RequireAuth(d.Sessions), func(c *gin.Context) { createGroup(c, d) })
	// 获取用户组详情
	api.GET("/groups/:gid", auth.RequireAuth(d.Sessions), func(c *gin.Context) { getGroup(c, d) })
	// 更新用户组信息
	api.PUT("/groups/:gid", auth.RequireAuth(d.Sessions), func(c *gin.Context) { updateGroup(c, d) })
	// 删除用户组
	api.DELETE("/groups/:gid", auth.RequireAuth(d.Sessions), func(c *gin.Context) { deleteGroup(c, d) })
	// 列出用户组成员
	api.GET("/groups/:gid/members", auth.RequireAuth(d.Sessions), func(c *gin.Context) { listGroupMembers(c, d) })
	// 添加用户组成员
	api.POST("/groups/:gid/members", auth.RequireAuth(d.Sessions), func(c *gin.Context) { addGroupMember(c, d) })
	// 更新用户组成员角色
	api.PUT("/groups/:gid/members/:uid", auth.RequireAuth(d.Sessions), func(c *gin.Context) { updateGroupMember(c, d) })
	// 移除用户组成员
	api.DELETE("/groups/:gid/members/:uid", auth.RequireAuth(d.Sessions), func(c *gin.Context) { removeGroupMember(c, d) })
	// 列出项目关联的多仓库
	api.GET("/projects/:id/repositories", auth.RequireAuth(d.Sessions), auth.RequireProject(d.IAM, iam.RoleGuest), func(c *gin.Context) { listRepositories(c, d) })
	// 为项目添加仓库
	api.POST("/projects/:id/repositories", auth.RequireAuth(d.Sessions), auth.RequireProject(d.IAM, iam.RoleMaintainer), func(c *gin.Context) { createRepository(c, d) })
	// 删除项目仓库
	api.DELETE("/projects/:id/repositories/:rid", auth.RequireAuth(d.Sessions), auth.RequireProject(d.IAM, iam.RoleMaintainer), func(c *gin.Context) { deleteRepository(c, d) })
	// 获取 Run 详情
	api.GET("/projects/:id/runs/:runId", auth.RequireAuth(d.Sessions), auth.RequireProject(d.IAM, iam.RoleGuest), func(c *gin.Context) { getRun(c, d) })
	// 获取 Run 审计报告内容
	api.GET("/projects/:id/runs/:runId/audit", auth.RequireAuth(d.Sessions), auth.RequireProject(d.IAM, iam.RoleGuest), func(c *gin.Context) { getRunAudit(c, d) })
	// 列出当前用户通知
	api.GET("/notifications", auth.RequireAuth(d.Sessions), func(c *gin.Context) { listNotifications(c, d) })
	// 获取未读通知数量
	api.GET("/notifications/unread_count", auth.RequireAuth(d.Sessions), func(c *gin.Context) { notificationUnreadCount(c, d) })
	// 标记单条通知为已读
	api.POST("/notifications/:nid/read", auth.RequireAuth(d.Sessions), func(c *gin.Context) { markNotificationRead(c, d) })
	// 将全部通知标记为已读
	api.POST("/notifications/read_all", auth.RequireAuth(d.Sessions), func(c *gin.Context) { markAllNotificationsRead(c, d) })
	// SSE 订阅实时通知推送
	api.GET("/notifications/stream", auth.RequireAuth(d.Sessions), func(c *gin.Context) { streamNotifications(c, d) })
}

// groupID 从路由参数解析用户组 ID。
func groupID(c *gin.Context) uuid.UUID {
	id, err := uuid.Parse(c.Param("gid"))
	if err != nil {
		return uuid.Nil
	}
	return id
}

// requireGroup 校验当前用户对用户组是否具备最低角色权限。
func requireGroup(c *gin.Context, d *app.Deps, min iam.Role) bool {
	u, ok := auth.User(c)
	if !ok {
		return false
	}
	gid := groupID(c)
	if gid == uuid.Nil {
		platformhttp.JSONError(c, 400, "bad_request", "无效的 ID")
		c.Abort()
		return false
	}
	allowed, err := d.IAM.CanAccessGroup(c.Request.Context(), u.ID, gid, min, u.IsAdmin)
	if err != nil || !allowed {
		platformhttp.JSONError(c, 403, "forbidden", "无权访问该组")
		c.Abort()
		return false
	}
	return true
}

// listGroups 列出Groups。
func listGroups(c *gin.Context, d *app.Deps) {
	u, _ := auth.User(c)
	list, err := d.Groups.ListForUser(c.Request.Context(), u.ID, u.IsAdmin)
	if err != nil {
		platformhttp.JSONError(c, 500, "internal", err.Error())
		return
	}
	c.JSON(200, gin.H{"groups": list})
}

// createGroup 创建Group。
func createGroup(c *gin.Context, d *app.Deps) {
	u, _ := auth.User(c)
	var in group.CreateInput
	if c.BindJSON(&in) != nil {
		platformhttp.JSONError(c, 400, "bad_request", "无效请求")
		return
	}
	g, err := d.Groups.Create(c.Request.Context(), u.ID, in)
	if err != nil {
		platformhttp.JSONError(c, 400, "bad_request", err.Error())
		return
	}
	c.JSON(201, g)
}

// getGroup 获取Group。
func getGroup(c *gin.Context, d *app.Deps) {
	if !requireGroup(c, d, iam.RoleGuest) {
		return
	}
	u, _ := auth.User(c)
	g, err := d.Groups.GetForUser(c.Request.Context(), groupID(c), u.ID, u.IsAdmin)
	if err != nil {
		platformhttp.JSONError(c, 404, "not_found", "组不存在")
		return
	}
	c.JSON(200, g)
}

// updateGroup 更新Group。
func updateGroup(c *gin.Context, d *app.Deps) {
	if !requireGroup(c, d, iam.RoleMaintainer) {
		return
	}
	u, _ := auth.User(c)
	gid := groupID(c)
	var body struct {
		Name *string `json:"name"`
	}
	if !bindJSON(c, &body) {
		return
	}
	if _, err := d.Groups.Update(c.Request.Context(), gid, body.Name); err != nil {
		platformhttp.JSONError(c, 400, "bad_request", err.Error())
		return
	}
	g, err := d.Groups.GetForUser(c.Request.Context(), gid, u.ID, u.IsAdmin)
	if err != nil {
		platformhttp.JSONError(c, 404, "not_found", "组不存在")
		return
	}
	c.JSON(200, g)
}

// deleteGroup 删除Group。
func deleteGroup(c *gin.Context, d *app.Deps) {
	if !requireGroup(c, d, iam.RoleOwner) {
		return
	}
	if err := d.Groups.Delete(c.Request.Context(), groupID(c)); err != nil {
		platformhttp.JSONError(c, 500, "internal", err.Error())
		return
	}
	c.JSON(200, gin.H{"ok": true})
}

// listGroupMembers 列出GroupMembers。
func listGroupMembers(c *gin.Context, d *app.Deps) {
	if !requireGroup(c, d, iam.RoleGuest) {
		return
	}
	members, err := d.Groups.ListMembers(c.Request.Context(), groupID(c))
	if err != nil {
		platformhttp.JSONError(c, 500, "internal", err.Error())
		return
	}
	c.JSON(200, gin.H{"members": members})
}

// addGroupMember 添加GroupMember。
func addGroupMember(c *gin.Context, d *app.Deps) {
	if !requireGroup(c, d, iam.RoleMaintainer) {
		return
	}
	var body struct {
		UserID uuid.UUID `json:"user_id"`
		Role   iam.Role  `json:"role"`
	}
	if c.BindJSON(&body) != nil {
		platformhttp.JSONError(c, 400, "bad_request", "无效请求")
		return
	}
	if err := d.Groups.AddMember(c.Request.Context(), groupID(c), body.UserID, body.Role); err != nil {
		platformhttp.JSONError(c, 400, "bad_request", err.Error())
		return
	}
	c.JSON(200, gin.H{"ok": true})
}

// updateGroupMember 更新GroupMember。
func updateGroupMember(c *gin.Context, d *app.Deps) {
	if !requireGroup(c, d, iam.RoleMaintainer) {
		return
	}
	uid, ok := paramUUID(c, "uid")
	if !ok {
		return
	}
	var body struct {
		Role iam.Role `json:"role"`
	}
	if !bindJSON(c, &body) {
		return
	}
	if err := d.Groups.UpdateMember(c.Request.Context(), groupID(c), uid, body.Role); err != nil {
		platformhttp.JSONError(c, 400, "bad_request", err.Error())
		return
	}
	c.JSON(200, gin.H{"ok": true})
}

// removeGroupMember 移除GroupMember。
func removeGroupMember(c *gin.Context, d *app.Deps) {
	if !requireGroup(c, d, iam.RoleMaintainer) {
		return
	}
	uid, ok := paramUUID(c, "uid")
	if !ok {
		return
	}
	if err := d.Groups.RemoveMember(c.Request.Context(), groupID(c), uid); err != nil {
		platformhttp.JSONError(c, 400, "bad_request", err.Error())
		return
	}
	c.JSON(200, gin.H{"ok": true})
}

// listRepositories 列出Repositories。
func listRepositories(c *gin.Context, d *app.Deps) {
	pid := auth.ProjectID(c)
	list, err := d.Repositories.List(c.Request.Context(), pid)
	if err != nil {
		platformhttp.JSONError(c, 500, "internal", err.Error())
		return
	}
	c.JSON(200, gin.H{"repositories": list})
}

// createRepository 创建Repository。
func createRepository(c *gin.Context, d *app.Deps) {
	pid := auth.ProjectID(c)
	var in repository.CreateInput
	if c.BindJSON(&in) != nil {
		platformhttp.JSONError(c, 400, "bad_request", "无效请求")
		return
	}
	r, err := d.Repositories.Create(c.Request.Context(), pid, in)
	if err != nil {
		platformhttp.JSONError(c, 400, "bad_request", err.Error())
		return
	}
	c.JSON(201, r)
}

// deleteRepository 删除Repository。
func deleteRepository(c *gin.Context, d *app.Deps) {
	pid := auth.ProjectID(c)
	rid, ok := paramUUID(c, "rid")
	if !ok {
		return
	}
	if err := d.Repositories.DeleteForProject(c.Request.Context(), pid, rid); err != nil {
		platformhttp.JSONError(c, 400, "bad_request", err.Error())
		return
	}
	c.JSON(200, gin.H{"ok": true})
}

// getRun 获取Run。
func getRun(c *gin.Context, d *app.Deps) {
	pid := auth.ProjectID(c)
	rid, ok := paramUUID(c, "runId")
	if !ok {
		return
	}
	rn, err := d.RunService.GetForProject(c.Request.Context(), pid, rid)
	if err != nil {
		platformhttp.JSONError(c, 404, "not_found", "运行不存在")
		return
	}
	c.JSON(200, rn)
}

// getRunAudit 获取RunAudit。
func getRunAudit(c *gin.Context, d *app.Deps) {
	pid := auth.ProjectID(c)
	rid, ok := paramUUID(c, "runId")
	if !ok {
		return
	}
	content, err := d.RunService.GetAuditForProject(c.Request.Context(), pid, rid)
	if err != nil {
		if os.IsNotExist(err) {
			c.JSON(200, gin.H{"content": ""})
			return
		}
		platformhttp.JSONError(c, 500, "internal", err.Error())
		return
	}
	c.JSON(200, gin.H{"content": content})
}

// listNotifications 列出Notifications。
func listNotifications(c *gin.Context, d *app.Deps) {
	u, _ := auth.User(c)
	list, err := d.Notifications.List(c.Request.Context(), u.ID, 50)
	if err != nil {
		platformhttp.JSONError(c, 500, "internal", err.Error())
		return
	}
	c.JSON(200, gin.H{"notifications": list})
}

// notificationUnreadCount 返回当前用户未读通知数量。
func notificationUnreadCount(c *gin.Context, d *app.Deps) {
	u, _ := auth.User(c)
	n, err := d.Notifications.UnreadCount(c.Request.Context(), u.ID)
	if err != nil {
		platformhttp.JSONError(c, 500, "internal", err.Error())
		return
	}
	c.JSON(200, gin.H{"count": n})
}

// markNotificationRead 标记NotificationRead。
func markNotificationRead(c *gin.Context, d *app.Deps) {
	u, _ := auth.User(c)
	nid, ok := paramUUID(c, "nid")
	if !ok {
		return
	}
	if err := d.Notifications.MarkRead(c.Request.Context(), u.ID, nid); err != nil {
		platformhttp.JSONError(c, 400, "bad_request", err.Error())
		return
	}
	c.JSON(200, gin.H{"ok": true})
}

// markAllNotificationsRead 将当前用户全部通知标记为已读。
func markAllNotificationsRead(c *gin.Context, d *app.Deps) {
	u, _ := auth.User(c)
	if err := d.Notifications.MarkAllRead(c.Request.Context(), u.ID); err != nil {
		platformhttp.JSONError(c, 500, "internal", err.Error())
		return
	}
	c.JSON(200, gin.H{"ok": true})
}

// streamNotifications 通过 SSE 推送Notifications。
func streamNotifications(c *gin.Context, d *app.Deps) {
	u, _ := auth.User(c)
	ch := d.Hub.Subscribe("user:" + u.ID.String())
	defer d.Hub.Unsubscribe("user:"+u.ID.String(), ch)
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		return
	}
	for {
		select {
		case msg, open := <-ch:
			if !open {
				return
			}
			if msg.Type != "notification" || msg.Output == "" {
				continue
			}
			fmt.Fprintf(c.Writer, "event: %s\ndata: %s\n\n", events.EventNotification, msg.Output)
			flusher.Flush()
		case <-c.Request.Context().Done():
			return
		}
	}
}
