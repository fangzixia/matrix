package routers

import (
	"encoding/json"
	"fmt"
	"matrix/internal/app"
	"matrix/internal/modules/iam"
	"matrix/internal/modules/run"
	"matrix/internal/platform/auth"
	"matrix/internal/platform/events"
	platformhttp "matrix/internal/platform/http"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// registerRunRoutes 注册 Run 任务与 SSE 流 API。
func registerRunRoutes(api *gin.RouterGroup, d *app.Deps) {
	authz := auth.RequireAuth(d.Sessions)
	guest := auth.RequireProject(d.IAM, iam.RoleGuest)
	dev := auth.RequireProject(d.IAM, iam.RoleDeveloper)
	api.GET("/projects/:id/runs", authz, guest, func(c *gin.Context) { listRuns(c, d) })
	api.POST("/projects/:id/runs", authz, dev, func(c *gin.Context) { startRun(c, d) })
	api.GET("/projects/:id/runs/:runId/stream", authz, guest, func(c *gin.Context) { streamRun(c, d) })
	api.POST("/projects/:id/runs/:runId/cancel", authz, dev, func(c *gin.Context) { cancelRun(c, d) })
	api.POST("/projects/:id/runs/:runId/merge", authz, dev, func(c *gin.Context) { mergeRun(c, d) })
	api.POST("/projects/:id/runs/:runId/discard", authz, dev, func(c *gin.Context) { discardRun(c, d) })
}

// listRuns 列出Runs。
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

// startRun 启动Run。
func startRun(c *gin.Context, d *app.Deps) {
	u, _ := auth.User(c)
	pid := auth.ProjectID(c)
	var body struct {
		Kind         string `json:"kind"`
		Message      string `json:"message"`
		FilePath     string `json:"file_path"`
		EvalFilePath string `json:"eval_file_path"`
	}
	_ = c.BindJSON(&body)
	if body.Kind == "" {
		body.Kind = "task"
	}
	sync := c.Query("sync") == "1"
	rn, err := d.Runs.Start(c.Request.Context(), pid, u.ID, run.StartInput{
		Kind: body.Kind, Message: body.Message, FilePath: body.FilePath,
		EvalFilePath: body.EvalFilePath, Title: body.Message, Sync: sync,
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

// cancelRun 取消Run。
func cancelRun(c *gin.Context, d *app.Deps) {
	rid, _ := uuid.Parse(c.Param("runId"))
	_ = d.Runs.Cancel(c.Request.Context(), rid)
	c.JSON(200, gin.H{"ok": true})
}

// mergeRun 合并Run。
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

// discardRun 丢弃Run。
func discardRun(c *gin.Context, d *app.Deps) {
	rid, _ := uuid.Parse(c.Param("runId"))
	rn, err := d.Runs.DiscardRun(c.Request.Context(), rid)
	if err != nil {
		platformhttp.JSONError(c, 400, "bad_request", err.Error())
		return
	}
	c.JSON(200, rn)
}
