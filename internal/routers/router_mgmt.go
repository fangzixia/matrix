package routers

import (
	"encoding/json"
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
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// registerMgmtRoutes 注册用户组、多仓库与 Run 详情等管理 API。
func registerMgmtRoutes(api *gin.RouterGroup, d *app.Deps) {
	api.GET("/groups", auth.RequireAuth(d.Sessions), func(c *gin.Context) { listGroups(c, d) })
	api.POST("/groups", auth.RequireAuth(d.Sessions), func(c *gin.Context) { createGroup(c, d) })
	api.GET("/groups/:gid", auth.RequireAuth(d.Sessions), func(c *gin.Context) { getGroup(c, d) })
	api.PUT("/groups/:gid", auth.RequireAuth(d.Sessions), func(c *gin.Context) { updateGroup(c, d) })
	api.DELETE("/groups/:gid", auth.RequireAuth(d.Sessions), func(c *gin.Context) { deleteGroup(c, d) })
	api.GET("/groups/:gid/members", auth.RequireAuth(d.Sessions), func(c *gin.Context) { listGroupMembers(c, d) })
	api.POST("/groups/:gid/members", auth.RequireAuth(d.Sessions), func(c *gin.Context) { addGroupMember(c, d) })
	api.PUT("/groups/:gid/members/:uid", auth.RequireAuth(d.Sessions), func(c *gin.Context) { updateGroupMember(c, d) })
	api.DELETE("/groups/:gid/members/:uid", auth.RequireAuth(d.Sessions), func(c *gin.Context) { removeGroupMember(c, d) })
	api.GET("/projects/:id/repositories", auth.RequireAuth(d.Sessions), auth.RequireProject(d.IAM, iam.RoleGuest), func(c *gin.Context) { listRepositories(c, d) })
	api.POST("/projects/:id/repositories", auth.RequireAuth(d.Sessions), auth.RequireProject(d.IAM, iam.RoleMaintainer), func(c *gin.Context) { createRepository(c, d) })
	api.DELETE("/projects/:id/repositories/:rid", auth.RequireAuth(d.Sessions), auth.RequireProject(d.IAM, iam.RoleMaintainer), func(c *gin.Context) { deleteRepository(c, d) })
	api.POST("/projects/:id/repositories/:rid/pull", auth.RequireAuth(d.Sessions), auth.RequireProject(d.IAM, iam.RoleDeveloper), func(c *gin.Context) { pullRepository(c, d) })
	api.POST("/projects/:id/repositories/:rid/push", auth.RequireAuth(d.Sessions), auth.RequireProject(d.IAM, iam.RoleMaintainer), func(c *gin.Context) { pushRepository(c, d) })
	api.GET("/projects/:id/runs/:runId", auth.RequireAuth(d.Sessions), auth.RequireProject(d.IAM, iam.RoleGuest), func(c *gin.Context) { getRun(c, d) })
	api.GET("/projects/:id/runs/:runId/steps", auth.RequireAuth(d.Sessions), auth.RequireProject(d.IAM, iam.RoleGuest), func(c *gin.Context) { listRunSteps(c, d) })
	api.GET("/projects/:id/runs/:runId/events", auth.RequireAuth(d.Sessions), auth.RequireProject(d.IAM, iam.RoleGuest), func(c *gin.Context) { listRunEvents(c, d) })
	api.GET("/projects/:id/runs/:runId/audit", auth.RequireAuth(d.Sessions), auth.RequireProject(d.IAM, iam.RoleGuest), func(c *gin.Context) { getRunAudit(c, d) })
	api.GET("/notifications", auth.RequireAuth(d.Sessions), func(c *gin.Context) { listNotifications(c, d) })
	api.GET("/notifications/unread_count", auth.RequireAuth(d.Sessions), func(c *gin.Context) { notificationUnreadCount(c, d) })
	api.POST("/notifications/:nid/read", auth.RequireAuth(d.Sessions), func(c *gin.Context) { markNotificationRead(c, d) })
	api.POST("/notifications/read_all", auth.RequireAuth(d.Sessions), func(c *gin.Context) { markAllNotificationsRead(c, d) })
	api.GET("/notifications/stream", auth.RequireAuth(d.Sessions), func(c *gin.Context) { streamNotifications(c, d) })
}

// groupID 从路由参数解析用户组 ID。
func groupID(c *gin.Context) uuid.UUID {
	id, _ := uuid.Parse(c.Param("gid"))
	return id
}

// requireGroup 校验当前用户对用户组是否具备最低角色权限。
func requireGroup(c *gin.Context, d *app.Deps, min iam.Role) bool {
	u, ok := auth.User(c)
	if !ok {
		return false
	}
	gid := groupID(c)
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
	_ = c.BindJSON(&body)
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
	uid, _ := uuid.Parse(c.Param("uid"))
	var body struct {
		Role iam.Role `json:"role"`
	}
	_ = c.BindJSON(&body)
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
	uid, _ := uuid.Parse(c.Param("uid"))
	_ = d.Groups.RemoveMember(c.Request.Context(), groupID(c), uid)
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
	_ = d.Workspace.EnsureRepo(c.Request.Context(), pid, r.Name, r.GitURL, r.GitBranch)
	c.JSON(201, r)
}

// deleteRepository 删除Repository。
func deleteRepository(c *gin.Context, d *app.Deps) {
	rid, _ := uuid.Parse(c.Param("rid"))
	if err := d.Repositories.Delete(c.Request.Context(), rid); err != nil {
		platformhttp.JSONError(c, 400, "bad_request", err.Error())
		return
	}
	c.JSON(200, gin.H{"ok": true})
}

// pullRepository 拉取Repository。
func pullRepository(c *gin.Context, d *app.Deps) {
	pid := auth.ProjectID(c)
	rid, _ := uuid.Parse(c.Param("rid"))
	if err := d.Workspace.PullByID(c.Request.Context(), pid, rid); err != nil {
		platformhttp.JSONError(c, 500, "internal", err.Error())
		return
	}
	c.JSON(200, gin.H{"ok": true})
}

// pushRepository 推送Repository。
func pushRepository(c *gin.Context, d *app.Deps) {
	pid := auth.ProjectID(c)
	rid, _ := uuid.Parse(c.Param("rid"))
	var body struct {
		Message string `json:"message"`
	}
	_ = c.BindJSON(&body)
	if err := d.Workspace.PushByID(c.Request.Context(), pid, rid, body.Message); err != nil {
		platformhttp.JSONError(c, 500, "internal", err.Error())
		return
	}
	c.JSON(200, gin.H{"ok": true})
}

// getRun 获取Run。
func getRun(c *gin.Context, d *app.Deps) {
	rid, _ := uuid.Parse(c.Param("runId"))
	rn, err := d.Runs.Get(c.Request.Context(), rid)
	if err != nil {
		platformhttp.JSONError(c, 404, "not_found", "运行不存在")
		return
	}
	c.JSON(200, rn)
}

// listRunSteps 列出RunSteps。
func listRunSteps(c *gin.Context, d *app.Deps) {
	rid, _ := uuid.Parse(c.Param("runId"))
	steps, err := d.Runs.ListSteps(c.Request.Context(), rid)
	if err != nil {
		platformhttp.JSONError(c, 500, "internal", err.Error())
		return
	}
	c.JSON(200, gin.H{"steps": steps})
}

// listRunEvents 列出RunEvents。
func listRunEvents(c *gin.Context, d *app.Deps) {
	rid, _ := uuid.Parse(c.Param("runId"))
	var afterID *uuid.UUID
	if s := c.Query("after_id"); s != "" {
		id, err := uuid.Parse(s)
		if err == nil {
			afterID = &id
		}
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "200"))
	events, err := d.Runs.ListEvents(c.Request.Context(), rid, afterID, limit)
	if err != nil {
		platformhttp.JSONError(c, 500, "internal", err.Error())
		return
	}
	c.JSON(200, gin.H{"events": events})
}

// getRunAudit 获取RunAudit。
func getRunAudit(c *gin.Context, d *app.Deps) {
	rid, _ := uuid.Parse(c.Param("runId"))
	content, err := d.Runs.GetAudit(c.Request.Context(), rid)
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
	nid, _ := uuid.Parse(c.Param("nid"))
	_ = d.Notifications.MarkRead(c.Request.Context(), u.ID, nid)
	c.JSON(200, gin.H{"ok": true})
}

// markAllNotificationsRead 将当前用户全部通知标记为已读。
func markAllNotificationsRead(c *gin.Context, d *app.Deps) {
	u, _ := auth.User(c)
	_ = d.Notifications.MarkAllRead(c.Request.Context(), u.ID)
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
			eventName := events.EventAgentStream
			var data []byte
			if msg.Type == "notification" && msg.Output != "" {
				eventName = events.EventNotification
				data = []byte(msg.Output)
			} else {
				data, _ = json.Marshal(msg)
			}
			fmt.Fprintf(c.Writer, "event: %s\ndata: %s\n\n", eventName, data)
			flusher.Flush()
		case <-c.Request.Context().Done():
			return
		}
	}
}

// repoNameForQuery 从查询参数解析并校验仓库名称。
func repoNameForQuery(c *gin.Context, d *app.Deps, pid uuid.UUID) string {
	rid := c.Query("repository_id")
	if rid == "" {
		return ""
	}
	id, err := uuid.Parse(rid)
	if err != nil {
		return ""
	}
	r, err := d.Repositories.Get(c.Request.Context(), id)
	if err != nil || r.ProjectID != pid {
		return ""
	}
	return r.Name
}
