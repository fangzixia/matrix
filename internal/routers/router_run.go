package routers

import (
	"encoding/json"
	"fmt"
	"matrix/internal/app"
	"matrix/internal/modules/iam"
	"matrix/internal/modules/run"
	"matrix/internal/modules/run/view"
	"matrix/internal/platform/auth"
	platformhttp "matrix/internal/platform/http"
	"matrix/internal/platform/logging"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const sseEventRunView = "run:view"

// registerRunRoutes 注册 Run 任务与 SSE 流 API。
func registerRunRoutes(api *gin.RouterGroup, d *app.Deps) {
	authz := auth.RequireAuth(d.Sessions)
	guest := auth.RequireProject(d.IAM, iam.RoleGuest)
	dev := auth.RequireProject(d.IAM, iam.RoleDeveloper)
	api.GET("/projects/:id/runs", authz, guest, func(c *gin.Context) { listRuns(c, d) })
	api.POST("/projects/:id/runs", authz, dev, func(c *gin.Context) { startRun(c, d) })
	api.GET("/projects/:id/runs/:runId/stream", authz, guest, func(c *gin.Context) { streamRun(c, d) })
	api.GET("/projects/:id/runs/:runId/view", authz, guest, func(c *gin.Context) { getRunView(c, d) })
	api.GET("/projects/:id/runs/:runId/tools/:toolUseId/log", authz, guest, func(c *gin.Context) { getToolLog(c, d) })
	api.POST("/projects/:id/runs/:runId/cancel", authz, dev, func(c *gin.Context) { cancelRun(c, d) })
}

// listRuns 列出Runs。
func listRuns(c *gin.Context, d *app.Deps) {
	pid := auth.ProjectID(c)
	kind := c.Query("kind")
	runs, err := d.RunService.List(c.Request.Context(), pid, kind)
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
	if !bindJSON(c, &body) {
		return
	}
	if body.Kind == "" {
		body.Kind = "task"
	}
	sync := c.Query("sync") == "1"
	rn, err := d.RunService.Start(c.Request.Context(), pid, u.ID, run.StartInput{
		Kind: body.Kind, Message: body.Message, FilePath: body.FilePath,
		EvalFilePath: body.EvalFilePath, Title: body.Message, Sync: sync,
	})
	if err != nil {
		platformhttp.JSONError(c, 400, "bad_request", err.Error())
		return
	}
	c.JSON(202, rn)
}

// streamRun 通过 SSE 推送 Run 视图事件；只读 DB 轮询，不依赖进程内 Hub。
func streamRun(c *gin.Context, d *app.Deps) {
	pid := auth.ProjectID(c)
	rid, ok := paramUUID(c, "runId")
	if !ok {
		return
	}
	runID := rid.String()
	mode := view.Mode(c.Query("mode"))
	if mode != view.ModeChat {
		mode = view.ModeDetail
	}
	lastSeq := parseRunViewAfterSeq(c)

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		platformhttp.JSONError(c, 500, "internal", "不支持流式响应")
		return
	}

	const pollInterval = 2 * time.Second
	first := true

	for {
		envs, done, maxSeq, err := d.RunService.StreamCatchUpSinceForProject(c.Request.Context(), pid, rid, mode, lastSeq)
		if err != nil {
			if first {
				if err == gorm.ErrRecordNotFound {
					platformhttp.JSONError(c, 404, "not_found", "Run 不存在")
				} else {
					platformhttp.JSONError(c, 500, "internal", err.Error())
				}
				return
			}
		} else {
			if first {
				logging.Agent("run-view: SSE 已连接",
					"run_id", runID, "mode", mode, "after_seq", lastSeq, "catchup_events", len(envs),
				)
				first = false
			}
			for _, env := range envs {
				writeRunViewSSE(c.Writer, env)
			}
			if maxSeq > lastSeq {
				lastSeq = maxSeq
			}
			flusher.Flush()
			if done {
				logging.Agent("run-view: SSE 终态已送达", "run_id", runID, "mode", mode)
				return
			}
		}

		select {
		case <-time.After(pollInterval):
		case <-c.Request.Context().Done():
			logging.Agent("run-view: SSE 客户端断开", "run_id", runID, "mode", mode)
			return
		}
	}
}

func writeRunViewSSE(w http.ResponseWriter, env view.Envelope) {
	b, _ := json.Marshal(env)
	fmt.Fprintf(w, "id: %d\nevent: %s\ndata: %s\n\n", env.Seq, sseEventRunView, b)
}

func parseRunViewAfterSeq(c *gin.Context) int64 {
	raw := c.Query("afterSeq")
	if raw == "" {
		raw = c.GetHeader("Last-Event-ID")
	}
	if raw == "" {
		return 0
	}
	seq, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || seq < 0 {
		return 0
	}
	return seq
}

// getRunView 返回 Run 活动视图快照。
func getRunView(c *gin.Context, d *app.Deps) {
	pid := auth.ProjectID(c)
	rid, ok := paramUUID(c, "runId")
	if !ok {
		return
	}
	st, err := d.RunService.GetRunViewForProject(c.Request.Context(), pid, rid)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			platformhttp.JSONError(c, 404, "not_found", "Run 不存在")
			return
		}
		platformhttp.JSONError(c, 500, "internal", err.Error())
		return
	}
	c.JSON(200, gin.H{"state": st})
}

// getToolLog 返回工具 spill 日志。
func getToolLog(c *gin.Context, d *app.Deps) {
	pid := auth.ProjectID(c)
	rid, ok := paramUUID(c, "runId")
	if !ok {
		return
	}
	toolUseID := c.Param("toolUseId")
	content, err := d.RunService.GetToolLogForProject(c.Request.Context(), pid, rid, toolUseID)
	if err != nil {
		if os.IsNotExist(err) {
			platformhttp.JSONError(c, 404, "not_found", "工具日志不存在")
			return
		}
		platformhttp.JSONError(c, 500, "internal", err.Error())
		return
	}
	c.JSON(200, gin.H{"content": content})
}

// cancelRun 取消Run。
func cancelRun(c *gin.Context, d *app.Deps) {
	pid := auth.ProjectID(c)
	rid, ok := paramUUID(c, "runId")
	if !ok {
		return
	}
	if err := d.RunService.CancelForProject(c.Request.Context(), pid, rid); err != nil {
		platformhttp.JSONError(c, 404, "not_found", "运行不存在")
		return
	}
	c.JSON(200, gin.H{"ok": true})
}
