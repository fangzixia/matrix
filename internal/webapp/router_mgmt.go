package webapp

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"matrix/internal/app"
	"matrix/internal/modules/group"
	"matrix/internal/modules/iam"
	"matrix/internal/modules/repository"
	"matrix/internal/modules/run"
	"matrix/internal/platform/auth"
	"matrix/internal/platform/events"
	platformhttp "matrix/internal/platform/http"
)

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
	api.PUT("/projects/:id/repositories/:rid", auth.RequireAuth(d.Sessions), auth.RequireProject(d.IAM, iam.RoleMaintainer), func(c *gin.Context) { updateRepository(c, d) })
	api.DELETE("/projects/:id/repositories/:rid", auth.RequireAuth(d.Sessions), auth.RequireProject(d.IAM, iam.RoleMaintainer), func(c *gin.Context) { deleteRepository(c, d) })
	api.POST("/projects/:id/repositories/:rid/pull", auth.RequireAuth(d.Sessions), auth.RequireProject(d.IAM, iam.RoleDeveloper), func(c *gin.Context) { pullRepository(c, d) })
	api.POST("/projects/:id/repositories/:rid/push", auth.RequireAuth(d.Sessions), auth.RequireProject(d.IAM, iam.RoleMaintainer), func(c *gin.Context) { pushRepository(c, d) })

	api.GET("/projects/:id/runs/:runId", auth.RequireAuth(d.Sessions), auth.RequireProject(d.IAM, iam.RoleGuest), func(c *gin.Context) { getRun(c, d) })
	api.GET("/projects/:id/runs/:runId/steps", auth.RequireAuth(d.Sessions), auth.RequireProject(d.IAM, iam.RoleGuest), func(c *gin.Context) { listRunSteps(c, d) })
	api.GET("/projects/:id/runs/:runId/events", auth.RequireAuth(d.Sessions), auth.RequireProject(d.IAM, iam.RoleGuest), func(c *gin.Context) { listRunEvents(c, d) })
	api.GET("/projects/:id/runs/:runId/audit", auth.RequireAuth(d.Sessions), auth.RequireProject(d.IAM, iam.RoleGuest), func(c *gin.Context) { getRunAudit(c, d) })
	api.POST("/projects/:id/pipelines", auth.RequireAuth(d.Sessions), auth.RequireProject(d.IAM, iam.RoleDeveloper), func(c *gin.Context) { startPipeline(c, d) })

	api.GET("/notifications", auth.RequireAuth(d.Sessions), func(c *gin.Context) { listNotifications(c, d) })
	api.GET("/notifications/unread_count", auth.RequireAuth(d.Sessions), func(c *gin.Context) { notificationUnreadCount(c, d) })
	api.POST("/notifications/:nid/read", auth.RequireAuth(d.Sessions), func(c *gin.Context) { markNotificationRead(c, d) })
	api.POST("/notifications/read_all", auth.RequireAuth(d.Sessions), func(c *gin.Context) { markAllNotificationsRead(c, d) })
	api.GET("/notifications/stream", auth.RequireAuth(d.Sessions), func(c *gin.Context) { streamNotifications(c, d) })
}

func groupID(c *gin.Context) uuid.UUID {
	id, _ := uuid.Parse(c.Param("gid"))
	return id
}

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

func listGroups(c *gin.Context, d *app.Deps) {
	u, _ := auth.User(c)
	list, err := d.Groups.ListForUser(c.Request.Context(), u.ID, u.IsAdmin)
	if err != nil {
		platformhttp.JSONError(c, 500, "internal", err.Error())
		return
	}
	c.JSON(200, gin.H{"groups": list})
}

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

func getGroup(c *gin.Context, d *app.Deps) {
	if !requireGroup(c, d, iam.RoleGuest) {
		return
	}
	g, err := d.Groups.Get(c.Request.Context(), groupID(c))
	if err != nil {
		platformhttp.JSONError(c, 404, "not_found", "组不存在")
		return
	}
	c.JSON(200, g)
}

func updateGroup(c *gin.Context, d *app.Deps) {
	if !requireGroup(c, d, iam.RoleMaintainer) {
		return
	}
	var body struct {
		Name       *string `json:"name"`
		Visibility *string `json:"visibility"`
	}
	_ = c.BindJSON(&body)
	g, err := d.Groups.Update(c.Request.Context(), groupID(c), body.Name, body.Visibility)
	if err != nil {
		platformhttp.JSONError(c, 400, "bad_request", err.Error())
		return
	}
	c.JSON(200, g)
}

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

func removeGroupMember(c *gin.Context, d *app.Deps) {
	if !requireGroup(c, d, iam.RoleMaintainer) {
		return
	}
	uid, _ := uuid.Parse(c.Param("uid"))
	_ = d.Groups.RemoveMember(c.Request.Context(), groupID(c), uid)
	c.JSON(200, gin.H{"ok": true})
}

func listRepositories(c *gin.Context, d *app.Deps) {
	pid := auth.ProjectID(c)
	list, err := d.Repositories.List(c.Request.Context(), pid)
	if err != nil {
		platformhttp.JSONError(c, 500, "internal", err.Error())
		return
	}
	c.JSON(200, gin.H{"repositories": list})
}

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

func updateRepository(c *gin.Context, d *app.Deps) {
	rid, _ := uuid.Parse(c.Param("rid"))
	var in repository.UpdateInput
	_ = c.BindJSON(&in)
	r, err := d.Repositories.Update(c.Request.Context(), rid, in)
	if err != nil {
		platformhttp.JSONError(c, 400, "bad_request", err.Error())
		return
	}
	c.JSON(200, r)
}

func deleteRepository(c *gin.Context, d *app.Deps) {
	rid, _ := uuid.Parse(c.Param("rid"))
	if err := d.Repositories.Delete(c.Request.Context(), rid); err != nil {
		platformhttp.JSONError(c, 400, "bad_request", err.Error())
		return
	}
	c.JSON(200, gin.H{"ok": true})
}

func pullRepository(c *gin.Context, d *app.Deps) {
	pid := auth.ProjectID(c)
	rid, _ := uuid.Parse(c.Param("rid"))
	if err := d.Workspace.PullByID(c.Request.Context(), pid, rid); err != nil {
		platformhttp.JSONError(c, 500, "internal", err.Error())
		return
	}
	c.JSON(200, gin.H{"ok": true})
}

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

func getRun(c *gin.Context, d *app.Deps) {
	rid, _ := uuid.Parse(c.Param("runId"))
	rn, err := d.Runs.Get(c.Request.Context(), rid)
	if err != nil {
		platformhttp.JSONError(c, 404, "not_found", "运行不存在")
		return
	}
	c.JSON(200, rn)
}

func listRunSteps(c *gin.Context, d *app.Deps) {
	rid, _ := uuid.Parse(c.Param("runId"))
	steps, err := d.Runs.ListSteps(c.Request.Context(), rid)
	if err != nil {
		platformhttp.JSONError(c, 500, "internal", err.Error())
		return
	}
	c.JSON(200, gin.H{"steps": steps})
}

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

func startPipeline(c *gin.Context, d *app.Deps) {
	u, _ := auth.User(c)
	pid := auth.ProjectID(c)
	var body struct {
		Message      string     `json:"message"`
		RepositoryID *uuid.UUID `json:"repository_id"`
		Stages       []string   `json:"stages"`
	}
	_ = c.BindJSON(&body)
	sync := c.Query("sync") == "1"
	rn, err := d.Runs.Start(c.Request.Context(), pid, u.ID, run.StartInput{
		Kind: "pipeline", Title: body.Message, Message: body.Message,
		RepositoryID: body.RepositoryID, Stages: body.Stages, Sync: sync,
	})
	if err != nil {
		platformhttp.JSONError(c, 400, "bad_request", err.Error())
		return
	}
	c.JSON(202, rn)
}

func listNotifications(c *gin.Context, d *app.Deps) {
	u, _ := auth.User(c)
	list, err := d.Notifications.List(c.Request.Context(), u.ID, 50)
	if err != nil {
		platformhttp.JSONError(c, 500, "internal", err.Error())
		return
	}
	c.JSON(200, gin.H{"notifications": list})
}

func notificationUnreadCount(c *gin.Context, d *app.Deps) {
	u, _ := auth.User(c)
	n, err := d.Notifications.UnreadCount(c.Request.Context(), u.ID)
	if err != nil {
		platformhttp.JSONError(c, 500, "internal", err.Error())
		return
	}
	c.JSON(200, gin.H{"count": n})
}

func markNotificationRead(c *gin.Context, d *app.Deps) {
	u, _ := auth.User(c)
	nid, _ := uuid.Parse(c.Param("nid"))
	_ = d.Notifications.MarkRead(c.Request.Context(), u.ID, nid)
	c.JSON(200, gin.H{"ok": true})
}

func markAllNotificationsRead(c *gin.Context, d *app.Deps) {
	u, _ := auth.User(c)
	_ = d.Notifications.MarkAllRead(c.Request.Context(), u.ID)
	c.JSON(200, gin.H{"ok": true})
}

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
			b, _ := json.Marshal(msg)
			fmt.Fprintf(c.Writer, "event: %s\ndata: %s\n\n", events.EventAgentStream, b)
			flusher.Flush()
		case <-c.Request.Context().Done():
			return
		}
	}
}

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
