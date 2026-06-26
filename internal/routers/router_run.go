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
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
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

// streamRun 通过 SSE 推送 Run 视图事件；只读 DB 轮询，不依赖进程内 Hub。
func streamRun(c *gin.Context, d *app.Deps) {
	runID := c.Param("runId")
	rid, err := uuid.Parse(runID)
	if err != nil {
		platformhttp.JSONError(c, 400, "bad_request", "无效的 Run ID")
		return
	}
	mode := view.Mode(c.Query("mode"))
	if mode != view.ModeChat {
		mode = view.ModeDetail
	}

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
	lastSeq := int64(0)
	first := true

	for {
		envs, done, maxSeq, err := d.Runs.StreamCatchUpSince(c.Request.Context(), rid, mode, lastSeq)
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
				logging.Info("run-view: SSE 已连接",
					"run_id", runID, "mode", mode, "catchup_events", len(envs),
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
				logging.Info("run-view: SSE 终态已送达", "run_id", runID, "mode", mode)
				return
			}
		}

		select {
		case <-time.After(pollInterval):
		case <-c.Request.Context().Done():
			logging.Info("run-view: SSE 客户端断开", "run_id", runID, "mode", mode)
			return
		}
	}
}

func writeRunViewSSE(w http.ResponseWriter, env view.Envelope) {
	b, _ := json.Marshal(env)
	fmt.Fprintf(w, "event: %s\ndata: %s\n\n", sseEventRunView, b)
}

// getRunView 返回 Run 活动视图快照。
func getRunView(c *gin.Context, d *app.Deps) {
	rid, err := uuid.Parse(c.Param("runId"))
	if err != nil {
		platformhttp.JSONError(c, 400, "bad_request", "无效的 Run ID")
		return
	}
	st, err := d.Runs.GetRunView(c.Request.Context(), rid)
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
	rid, err := uuid.Parse(c.Param("runId"))
	if err != nil {
		platformhttp.JSONError(c, 400, "bad_request", "无效的 Run ID")
		return
	}
	toolUseID := c.Param("toolUseId")
	content, err := d.Runs.GetToolLog(c.Request.Context(), rid, toolUseID)
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
