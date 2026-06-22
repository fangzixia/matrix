// Package webapp 注册 Gin 路由并将 HTTP 请求适配到各领域 Service。
package webapp

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"matrix/internal/app"
	"matrix/internal/ai/query"
	"matrix/internal/modules/iam"
	"matrix/internal/modules/identity"
	"matrix/internal/modules/project"
	"matrix/internal/modules/run"
	"matrix/internal/modules/workspace"
	"matrix/internal/platform/auth"
	"matrix/internal/platform/events"
	platformhttp "matrix/internal/platform/http"
)

// Register 挂载 REST API、Admin 路由与前端静态资源（SPA fallback）。
func Register(r *gin.Engine, d *app.Deps, staticFS fs.FS) {
	r.GET("/health", func(c *gin.Context) { c.JSON(200, gin.H{"status": "ok"}) })

	api := r.Group("/api")
	{
		// --- 认证 ---
		api.POST("/auth/login", func(c *gin.Context) { login(c, d) })
		api.POST("/auth/logout", auth.RequireAuth(d.Sessions), func(c *gin.Context) { logout(c, d) })
		api.GET("/auth/me", auth.RequireAuth(d.Sessions), func(c *gin.Context) { me(c, d) })

		admin := api.Group("/admin", auth.RequireAuth(d.Sessions), auth.RequireAdmin())
		{
			// --- Admin 用户管理 ---
			admin.GET("/users", func(c *gin.Context) { adminListUsers(c, d) })
			admin.POST("/users", func(c *gin.Context) { adminCreateUser(c, d) })
			admin.GET("/users/:uid", func(c *gin.Context) { adminGetUser(c, d) })
			admin.PUT("/users/:uid", func(c *gin.Context) { adminUpdateUser(c, d) })
			admin.DELETE("/users/:uid", func(c *gin.Context) { adminDeleteUser(c, d) })
			admin.POST("/users/:uid/reset_password", func(c *gin.Context) { adminResetPassword(c, d) })
			admin.POST("/users/:uid/block", func(c *gin.Context) { adminBlockUser(c, d) })
			admin.POST("/users/:uid/unblock", func(c *gin.Context) { adminUnblockUser(c, d) })
			registerSystemAdminRoutes(admin, d) // root 系统配置（AI/MCP/Git/Worker/Pipeline）
		}

		api.GET("/users/search", auth.RequireAuth(d.Sessions), func(c *gin.Context) { searchUsers(c, d) })

		// --- 项目 ---
		api.GET("/projects", auth.RequireAuth(d.Sessions), func(c *gin.Context) { listProjects(c, d) })
		api.POST("/projects", auth.RequireAuth(d.Sessions), func(c *gin.Context) { createProject(c, d) })
		api.GET("/projects/:id", auth.RequireAuth(d.Sessions), auth.RequireProject(d.IAM, iam.RoleGuest), func(c *gin.Context) { getProject(c, d) })
		api.PUT("/projects/:id", auth.RequireAuth(d.Sessions), auth.RequireProject(d.IAM, iam.RoleMaintainer), func(c *gin.Context) { updateProject(c, d) })
		api.DELETE("/projects/:id", auth.RequireAuth(d.Sessions), auth.RequireProject(d.IAM, iam.RoleOwner), func(c *gin.Context) { deleteProject(c, d) })

		api.GET("/projects/:id/members", auth.RequireAuth(d.Sessions), auth.RequireProject(d.IAM, iam.RoleGuest), func(c *gin.Context) { listMembers(c, d) })
		api.POST("/projects/:id/members", auth.RequireAuth(d.Sessions), auth.RequireProject(d.IAM, iam.RoleMaintainer), func(c *gin.Context) { addMember(c, d) })
		api.PUT("/projects/:id/members/:uid", auth.RequireAuth(d.Sessions), auth.RequireProject(d.IAM, iam.RoleMaintainer), func(c *gin.Context) { updateMember(c, d) })
		api.DELETE("/projects/:id/members/:uid", auth.RequireAuth(d.Sessions), auth.RequireProject(d.IAM, iam.RoleMaintainer), func(c *gin.Context) { removeMember(c, d) })

		// --- Git 工作区 ---
		api.GET("/projects/:id/repository/tree", auth.RequireAuth(d.Sessions), auth.RequireProject(d.IAM, iam.RoleGuest), func(c *gin.Context) { listFiles(c, d) })
		api.GET("/projects/:id/repository/file", auth.RequireAuth(d.Sessions), auth.RequireProject(d.IAM, iam.RoleGuest), func(c *gin.Context) { readFile(c, d) })
		api.POST("/projects/:id/repository/pull", auth.RequireAuth(d.Sessions), auth.RequireProject(d.IAM, iam.RoleDeveloper), func(c *gin.Context) { pullRepo(c, d) })
		api.POST("/projects/:id/repository/push", auth.RequireAuth(d.Sessions), auth.RequireProject(d.IAM, iam.RoleMaintainer), func(c *gin.Context) { pushRepo(c, d) })

		api.GET("/projects/:id/settings/integrations", auth.RequireAuth(d.Sessions), auth.RequireProject(d.IAM, iam.RoleGuest), func(c *gin.Context) { getIntegrations(c, d) })
		api.PUT("/projects/:id/settings/integrations", auth.RequireAuth(d.Sessions), auth.RequireProject(d.IAM, iam.RoleMaintainer), func(c *gin.Context) { saveIntegrations(c, d) })

		api.GET("/projects/:id/plans", auth.RequireAuth(d.Sessions), auth.RequireProject(d.IAM, iam.RoleGuest), func(c *gin.Context) { listPlans(c, d) })
		api.GET("/projects/:id/evaluations", auth.RequireAuth(d.Sessions), auth.RequireProject(d.IAM, iam.RoleGuest), func(c *gin.Context) { listEvaluations(c, d) })

		// --- Run 任务与 SSE 流 ---
		api.GET("/projects/:id/runs", auth.RequireAuth(d.Sessions), auth.RequireProject(d.IAM, iam.RoleGuest), func(c *gin.Context) { listRuns(c, d) })
		api.POST("/projects/:id/runs", auth.RequireAuth(d.Sessions), auth.RequireProject(d.IAM, iam.RoleDeveloper), func(c *gin.Context) { startRun(c, d) })
		api.GET("/projects/:id/runs/:runId/stream", auth.RequireAuth(d.Sessions), auth.RequireProject(d.IAM, iam.RoleGuest), func(c *gin.Context) { streamRun(c, d) })
		api.POST("/projects/:id/runs/:runId/cancel", auth.RequireAuth(d.Sessions), auth.RequireProject(d.IAM, iam.RoleDeveloper), func(c *gin.Context) { cancelRun(c, d) })
		api.POST("/projects/:id/runs/:runId/merge", auth.RequireAuth(d.Sessions), auth.RequireProject(d.IAM, iam.RoleDeveloper), func(c *gin.Context) { mergeRun(c, d) })
		api.POST("/projects/:id/runs/:runId/discard", auth.RequireAuth(d.Sessions), auth.RequireProject(d.IAM, iam.RoleDeveloper), func(c *gin.Context) { discardRun(c, d) })

		// --- Chat 会话 ---
		api.GET("/projects/:id/chat/sessions", auth.RequireAuth(d.Sessions), auth.RequireProject(d.IAM, iam.RoleGuest), func(c *gin.Context) { listChat(c, d) })
		api.PUT("/projects/:id/chat/sessions", auth.RequireAuth(d.Sessions), auth.RequireProject(d.IAM, iam.RoleDeveloper), func(c *gin.Context) { saveChat(c, d) })
		api.POST("/projects/:id/chat/sessions/:sid/run", auth.RequireAuth(d.Sessions), auth.RequireProject(d.IAM, iam.RoleDeveloper), func(c *gin.Context) { runChat(c, d) })

		api.PUT("/profile", auth.RequireAuth(d.Sessions), func(c *gin.Context) { updateProfile(c, d) })

		registerMgmtRoutes(api, d)
	}

	if staticFS != nil {
		// 前端 SPA：/assets 静态资源，其余 GET 回退 index.html
		r.GET("/assets/*filepath", func(c *gin.Context) {
			fp := c.Param("filepath")
			data, err := fs.ReadFile(staticFS, "assets"+fp)
			if err != nil {
				c.Status(404)
				return
			}
			c.Data(200, contentType(fp), data)
		})
		r.NoRoute(func(c *gin.Context) {
			if c.Request.Method != http.MethodGet {
				c.Status(404)
				return
			}
			path := strings.TrimPrefix(c.Request.URL.Path, "/")
			if path == "" {
				path = "index.html"
			}
			data, err := fs.ReadFile(staticFS, path)
			if err != nil {
				data, _ = fs.ReadFile(staticFS, "index.html")
			}
			if data == nil {
				c.String(404, "not found")
				return
			}
			c.Data(http.StatusOK, contentType(path), data)
		})
	}
}

func contentType(path string) string {
	if len(path) > 4 && path[len(path)-4:] == ".css" {
		return "text/css"
	}
	if len(path) > 3 && path[len(path)-3:] == ".js" {
		return "application/javascript"
	}
	if len(path) > 4 && path[len(path)-4:] == ".svg" {
		return "image/svg+xml"
	}
	return "text/html"
}

func login(c *gin.Context, d *app.Deps) {
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if c.BindJSON(&body) != nil {
		platformhttp.JSONError(c, 400, "bad_request", "无效请求")
		return
	}
	u, token, err := d.Auth.Login(c.Request.Context(), body.Username, body.Password, c.ClientIP(), c.Request.UserAgent())
	if err != nil {
		platformhttp.JSONError(c, 401, "unauthorized", "用户名或密码错误")
		return
	}
	c.SetCookie(d.Sessions.CookieName(), token, int(d.Sessions.TTL().Seconds()), "/", "", d.Sessions.Secure(), true)
	c.JSON(200, userResponse(d, u))
}

func userResponse(d *app.Deps, u *identity.User) gin.H {
	rootUser := d.Config.Auth.Bootstrap.AdminUsername
	if rootUser == "" {
		rootUser = "root"
	}
	return gin.H{
		"id":              u.ID,
		"username":        u.Username,
		"email":           u.Email,
		"name":            u.Name,
		"avatar_url":      u.AvatarURL,
		"is_admin":        u.IsAdmin,
		"is_root":         u.Username == rootUser,
		"state":           u.State,
		"last_sign_in_at": u.LastSignInAt,
		"created_at":      u.CreatedAt,
	}
}

func logout(c *gin.Context, d *app.Deps) {
	token, _ := c.Cookie(d.Sessions.CookieName())
	_ = d.Auth.Logout(c.Request.Context(), token)
	c.SetCookie(d.Sessions.CookieName(), "", -1, "/", "", d.Sessions.Secure(), true)
	c.JSON(200, gin.H{"ok": true})
}

func me(c *gin.Context, d *app.Deps) {
	u, ok := auth.User(c)
	if !ok {
		platformhttp.JSONError(c, 401, "unauthorized", "未登录")
		return
	}
	c.JSON(200, userResponse(d, u))
}

func adminListUsers(c *gin.Context, d *app.Deps) {
	list, total, err := d.Users.ListWithStats(c.Request.Context(), 100, 0)
	if err != nil {
		platformhttp.JSONError(c, 500, "internal", err.Error())
		return
	}
	c.JSON(200, gin.H{"users": list, "total": total})
}

func adminCreateUser(c *gin.Context, d *app.Deps) {
	var in identity.CreateUserInput
	if c.BindJSON(&in) != nil {
		platformhttp.JSONError(c, 400, "bad_request", "无效请求")
		return
	}
	u, err := d.Users.Create(c.Request.Context(), in)
	if err != nil {
		platformhttp.JSONError(c, 400, "bad_request", err.Error())
		return
	}
	c.JSON(201, u)
}

func adminGetUser(c *gin.Context, d *app.Deps) {
	id, err := uuid.Parse(c.Param("uid"))
	if err != nil {
		platformhttp.JSONError(c, 400, "bad_request", "无效 ID")
		return
	}
	u, err := d.Users.GetByID(c.Request.Context(), id)
	if err != nil {
		platformhttp.JSONError(c, 404, "not_found", "用户不存在")
		return
	}
	c.JSON(200, u)
}

func adminUpdateUser(c *gin.Context, d *app.Deps) {
	id, _ := uuid.Parse(c.Param("uid"))
	var in identity.UpdateUserInput
	if c.BindJSON(&in) != nil {
		platformhttp.JSONError(c, 400, "bad_request", "无效请求")
		return
	}
	u, err := d.Users.Update(c.Request.Context(), id, in)
	if err != nil {
		platformhttp.JSONError(c, 400, "bad_request", err.Error())
		return
	}
	c.JSON(200, u)
}

func adminDeleteUser(c *gin.Context, d *app.Deps) {
	id, _ := uuid.Parse(c.Param("uid"))
	if err := d.Users.Delete(c.Request.Context(), id); err != nil {
		platformhttp.JSONError(c, 500, "internal", err.Error())
		return
	}
	c.JSON(200, gin.H{"ok": true})
}

func adminResetPassword(c *gin.Context, d *app.Deps) {
	id, err := uuid.Parse(c.Param("uid"))
	if err != nil {
		platformhttp.JSONError(c, 400, "bad_request", "无效 ID")
		return
	}
	var body struct {
		Password string `json:"password"`
	}
	if c.BindJSON(&body) != nil || body.Password == "" {
		platformhttp.JSONError(c, 400, "bad_request", "password 必填")
		return
	}
	if err := d.Users.ResetPassword(c.Request.Context(), id, body.Password); err != nil {
		platformhttp.JSONError(c, 500, "internal", err.Error())
		return
	}
	c.JSON(200, gin.H{"ok": true})
}

func adminBlockUser(c *gin.Context, d *app.Deps) {
	id, err := uuid.Parse(c.Param("uid"))
	if err != nil {
		platformhttp.JSONError(c, 400, "bad_request", "无效 ID")
		return
	}
	if err := d.Users.Block(c.Request.Context(), id); err != nil {
		platformhttp.JSONError(c, 500, "internal", err.Error())
		return
	}
	c.JSON(200, gin.H{"ok": true})
}

func adminUnblockUser(c *gin.Context, d *app.Deps) {
	id, err := uuid.Parse(c.Param("uid"))
	if err != nil {
		platformhttp.JSONError(c, 400, "bad_request", "无效 ID")
		return
	}
	if err := d.Users.Unblock(c.Request.Context(), id); err != nil {
		platformhttp.JSONError(c, 500, "internal", err.Error())
		return
	}
	c.JSON(200, gin.H{"ok": true})
}

func searchUsers(c *gin.Context, d *app.Deps) {
	q := c.Query("q")
	list, err := d.Users.Search(c.Request.Context(), q, 20)
	if err != nil {
		platformhttp.JSONError(c, 500, "internal", err.Error())
		return
	}
	c.JSON(200, gin.H{"users": list})
}

func listProjects(c *gin.Context, d *app.Deps) {
	u, _ := auth.User(c)
	scope := c.DefaultQuery("scope", "yours")
	list, err := d.Projects.ListForUser(c.Request.Context(), u.ID, u.IsAdmin, scope)
	if err != nil {
		platformhttp.JSONError(c, 500, "internal", err.Error())
		return
	}
	c.JSON(200, gin.H{"projects": list})
}

func createProject(c *gin.Context, d *app.Deps) {
	u, _ := auth.User(c)
	var in project.CreateInput
	if c.BindJSON(&in) != nil {
		platformhttp.JSONError(c, 400, "bad_request", "无效请求")
		return
	}
	p, err := d.Projects.Create(c.Request.Context(), u.ID, in)
	if err != nil {
		platformhttp.JSONError(c, 400, "bad_request", err.Error())
		return
	}
	_ = d.Repositories.SeedDefault(c.Request.Context(), p.ID, p.GitURL, p.GitBranch)
	_ = d.Workspace.EnsureClone(c.Request.Context(), p.ID, p.GitURL, p.GitBranch)
	c.JSON(201, p)
}

func getProject(c *gin.Context, d *app.Deps) {
	pid := auth.ProjectID(c)
	u, _ := auth.User(c)
	p, err := d.Projects.GetForUser(c.Request.Context(), pid, u.ID, u.IsAdmin)
	if err != nil {
		platformhttp.JSONError(c, 404, "not_found", "项目不存在")
		return
	}
	c.JSON(200, p)
}

func updateProject(c *gin.Context, d *app.Deps) {
	pid := auth.ProjectID(c)
	var body struct {
		Name       *string    `json:"name"`
		Path       *string    `json:"path"`
		GitURL     *string    `json:"git_url"`
		GitBranch  *string    `json:"git_branch"`
		Visibility *string    `json:"visibility"`
		GroupID    *uuid.UUID `json:"group_id"`
	}
	_ = c.BindJSON(&body)
	p, err := d.Projects.Update(c.Request.Context(), pid, body.Name, body.Path, body.GitURL, body.GitBranch, body.Visibility, body.GroupID)
	if err != nil {
		platformhttp.JSONError(c, 400, "bad_request", err.Error())
		return
	}
	c.JSON(200, p)
}

func deleteProject(c *gin.Context, d *app.Deps) {
	pid := auth.ProjectID(c)
	if err := d.Projects.Delete(c.Request.Context(), pid); err != nil {
		platformhttp.JSONError(c, 500, "internal", err.Error())
		return
	}
	c.JSON(200, gin.H{"ok": true})
}

func listMembers(c *gin.Context, d *app.Deps) {
	pid := auth.ProjectID(c)
	list, err := d.Members.List(c.Request.Context(), pid)
	if err != nil {
		platformhttp.JSONError(c, 500, "internal", err.Error())
		return
	}
	c.JSON(200, gin.H{"members": list})
}

func addMember(c *gin.Context, d *app.Deps) {
	pid := auth.ProjectID(c)
	var body struct {
		UserID   *uuid.UUID `json:"user_id"`
		Username *string    `json:"username"`
		Role     iam.Role   `json:"role"`
	}
	if c.BindJSON(&body) != nil {
		platformhttp.JSONError(c, 400, "bad_request", "无效请求")
		return
	}
	uid := uuid.Nil
	if body.UserID != nil && *body.UserID != uuid.Nil {
		uid = *body.UserID
	} else if body.Username != nil && *body.Username != "" {
		u, err := d.Users.GetByUsername(c.Request.Context(), *body.Username)
		if err != nil {
			platformhttp.JSONError(c, 404, "not_found", "用户不存在")
			return
		}
		uid = u.ID
	} else {
		platformhttp.JSONError(c, 400, "bad_request", "user_id 或 username 必填")
		return
	}
	if err := d.Members.Add(c.Request.Context(), pid, uid, body.Role); err != nil {
		platformhttp.JSONError(c, 400, "bad_request", err.Error())
		return
	}
	c.JSON(201, gin.H{"ok": true})
}

func updateMember(c *gin.Context, d *app.Deps) {
	pid := auth.ProjectID(c)
	uid, _ := uuid.Parse(c.Param("uid"))
	var body struct {
		Role iam.Role `json:"role"`
	}
	_ = c.BindJSON(&body)
	if err := d.Members.UpdateRole(c.Request.Context(), pid, uid, body.Role); err != nil {
		platformhttp.JSONError(c, 400, "bad_request", err.Error())
		return
	}
	c.JSON(200, gin.H{"ok": true})
}

func removeMember(c *gin.Context, d *app.Deps) {
	pid := auth.ProjectID(c)
	uid, _ := uuid.Parse(c.Param("uid"))
	_ = d.Members.Remove(c.Request.Context(), pid, uid)
	c.JSON(200, gin.H{"ok": true})
}

func listFiles(c *gin.Context, d *app.Deps) {
	pid := auth.ProjectID(c)
	path := c.Query("path")
	repoName := repoNameForQuery(c, d, pid)
	var files []workspace.FileEntry
	var err error
	if repoName != "" {
		files, err = d.Workspace.ListFilesFor(pid, repoName, path)
	} else {
		files, err = d.Workspace.ListFiles(pid, path)
	}
	if err != nil {
		platformhttp.JSONError(c, 500, "internal", err.Error())
		return
	}
	c.JSON(200, gin.H{"files": files})
}

func readFile(c *gin.Context, d *app.Deps) {
	pid := auth.ProjectID(c)
	repoName := repoNameForQuery(c, d, pid)
	var content string
	var err error
	if repoName != "" {
		content, err = d.Workspace.ReadFileFor(pid, repoName, c.Query("path"))
	} else {
		content, err = d.Workspace.ReadFile(pid, c.Query("path"))
	}
	if err != nil {
		platformhttp.JSONError(c, 500, "internal", err.Error())
		return
	}
	c.JSON(200, gin.H{"content": content})
}

func listPlans(c *gin.Context, d *app.Deps) {
	pid := auth.ProjectID(c)
	repoID := parseRepositoryIDQuery(c)
	items, err := d.Plans.List(c.Request.Context(), pid, repoID)
	if err != nil {
		platformhttp.JSONError(c, 500, "internal", err.Error())
		return
	}
	c.JSON(200, gin.H{"plans": items})
}

func listEvaluations(c *gin.Context, d *app.Deps) {
	pid := auth.ProjectID(c)
	repoID := parseRepositoryIDQuery(c)
	items, err := d.Artifacts.ListEvaluations(c.Request.Context(), pid, repoID)
	if err != nil {
		platformhttp.JSONError(c, 500, "internal", err.Error())
		return
	}
	c.JSON(200, gin.H{"evaluations": items})
}

func parseRepositoryIDQuery(c *gin.Context) *uuid.UUID {
	raw := c.Query("repository_id")
	if raw == "" {
		return nil
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		return nil
	}
	return &id
}

func listRuns(c *gin.Context, d *app.Deps) {
	pid := auth.ProjectID(c)
	kind := c.Query("kind")
	runs, err := d.Runs.List(c.Request.Context(), pid, kind)
	if err != nil {
		platformhttp.JSONError(c, 500, "internal", err.Error())
		return
	}
	c.JSON(200, gin.H{"runs": runs})
}

func startRun(c *gin.Context, d *app.Deps) {
	u, _ := auth.User(c)
	pid := auth.ProjectID(c)
	var body struct {
		Kind     string `json:"kind"`
		Message  string `json:"message"`
		FilePath string `json:"file_path"`
	}
	_ = c.BindJSON(&body)
	if body.Kind == "" {
		body.Kind = "task"
	}
	sync := c.Query("sync") == "1"
	rn, err := d.Runs.Start(c.Request.Context(), pid, u.ID, run.StartInput{
		Kind: body.Kind, Message: body.Message, FilePath: body.FilePath, Title: body.Message, Sync: sync,
	})
	if err != nil {
		platformhttp.JSONError(c, 400, "bad_request", err.Error())
		return
	}
	c.JSON(202, rn)
}

// streamRun 通过 SSE 推送 Run 执行事件（Agent 流式输出）。
func streamRun(c *gin.Context, d *app.Deps) {
	runID := c.Param("runId")
	ch := d.Hub.Subscribe(runID)
	defer d.Hub.Unsubscribe(runID, ch)
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

func cancelRun(c *gin.Context, d *app.Deps) {
	rid, _ := uuid.Parse(c.Param("runId"))
	_ = d.Runs.Cancel(c.Request.Context(), rid)
	c.JSON(200, gin.H{"ok": true})
}

func mergeRun(c *gin.Context, d *app.Deps) {
	rid, _ := uuid.Parse(c.Param("runId"))
	rn, conflicts, err := d.Runs.MergeRun(c.Request.Context(), rid)
	if err != nil {
		if len(conflicts) > 0 {
			c.JSON(409, gin.H{"error": err.Error(), "conflicts": conflicts, "run": rn})
			return
		}
		platformhttp.JSONError(c, 400, "bad_request", err.Error())
		return
	}
	c.JSON(200, rn)
}

func discardRun(c *gin.Context, d *app.Deps) {
	rid, _ := uuid.Parse(c.Param("runId"))
	rn, err := d.Runs.DiscardRun(c.Request.Context(), rid)
	if err != nil {
		platformhttp.JSONError(c, 400, "bad_request", err.Error())
		return
	}
	c.JSON(200, rn)
}

func listChat(c *gin.Context, d *app.Deps) {
	pid := auth.ProjectID(c)
	sessions, _ := d.Runs.ListChatSessions(c.Request.Context(), pid)
	c.JSON(200, gin.H{"sessions": sessions})
}

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

func updateProfile(c *gin.Context, d *app.Deps) {
	u, _ := auth.User(c)
	var in identity.UpdateUserInput
	if c.BindJSON(&in) != nil {
		platformhttp.JSONError(c, 400, "bad_request", "无效请求")
		return
	}
	in.IsAdmin = nil
	out, err := d.Users.Update(c.Request.Context(), u.ID, in)
	if err != nil {
		platformhttp.JSONError(c, 400, "bad_request", err.Error())
		return
	}
	c.JSON(200, out)
}

func getIntegrations(c *gin.Context, d *app.Deps) {
	pid := auth.ProjectID(c)
	s, err := d.Settings.Get(c.Request.Context(), pid)
	if err != nil {
		platformhttp.JSONError(c, 500, "internal", err.Error())
		return
	}
	c.JSON(200, s)
}

func saveIntegrations(c *gin.Context, d *app.Deps) {
	pid := auth.ProjectID(c)
	var in project.IntegrationSettings
	if c.BindJSON(&in) != nil {
		platformhttp.JSONError(c, 400, "bad_request", "无效请求")
		return
	}
	s, err := d.Settings.Save(c.Request.Context(), pid, in)
	if err != nil {
		platformhttp.JSONError(c, 400, "bad_request", err.Error())
		return
	}
	c.JSON(200, s)
}

func pullRepo(c *gin.Context, d *app.Deps) {
	pid := auth.ProjectID(c)
	if err := d.Workspace.Pull(c.Request.Context(), pid); err != nil {
		platformhttp.JSONError(c, 500, "internal", err.Error())
		return
	}
	c.JSON(200, gin.H{"ok": true})
}

func pushRepo(c *gin.Context, d *app.Deps) {
	pid := auth.ProjectID(c)
	var body struct {
		Message string `json:"message"`
	}
	_ = c.BindJSON(&body)
	if err := d.Workspace.Push(c.Request.Context(), pid, body.Message); err != nil {
		platformhttp.JSONError(c, 500, "internal", err.Error())
		return
	}
	c.JSON(200, gin.H{"ok": true})
}
