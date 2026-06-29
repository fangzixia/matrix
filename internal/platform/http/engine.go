// Package http 封装 Gin 引擎、请求日志与统一 JSON 错误响应。
package http

import (
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"

	"matrix/internal/platform/logging"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ErrorBody 是 API 统一错误响应 JSON 结构。
type ErrorBody struct {
	Error   string `json:"error"`
	Message string `json:"message"`
	Code    string `json:"code,omitempty"`
}

// NewEngine 创建带 Recovery、Request-ID、访问日志与 5xx 系统日志的 Gin 引擎。
func NewEngine(access *logging.AccessLog, system *slog.Logger, dev bool) *gin.Engine {
	if !dev {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.New()
	r.Use(recoveryMiddleware(system))
	r.Use(requestIDMiddleware())
	r.Use(accessLogMiddleware(access))
	r.Use(serverErrorLogMiddleware(system))
	return r
}

// recoveryMiddleware 捕获 panic 并写入 system 日志。
func recoveryMiddleware(system *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				if system != nil {
					system.Error("HTTP panic",
						"request_id", c.GetString("request_id"),
						"method", c.Request.Method,
						"path", c.Request.URL.Path,
						"error", r,
						"stack", string(debug.Stack()),
					)
				}
				c.AbortWithStatus(http.StatusInternalServerError)
			}
		}()
		c.Next()
	}
}

// serverErrorLogMiddleware 将 5xx 响应写入 system 日志。
func serverErrorLogMiddleware(system *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
		if system == nil || c.Writer.Status() < 500 {
			return
		}
		system.Warn("HTTP 5xx",
			"request_id", c.GetString("request_id"),
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
		)
	}
}

// requestIDMiddleware 为每个 HTTP 请求注入唯一 request ID。
func requestIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader("X-Request-ID")
		if id == "" {
			id = uuid.NewString()
		}
		c.Set("request_id", id)
		c.Header("X-Request-ID", id)
		c.Next()
	}
}

// accessLogMiddleware 记录 nginx combined 风格的 API 访问日志。
func accessLogMiddleware(access *logging.AccessLog) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		if access != nil {
			access.WriteCombined(c, time.Since(start))
		}
	}
}

// JSONError 以统一格式返回 JSON 错误并中止后续 Handler。
func JSONError(c *gin.Context, status int, code, message string) {
	c.AbortWithStatusJSON(status, ErrorBody{Error: code, Message: message, Code: code})
}

// ServeStatic 挂载静态资源并在未匹配 GET 路由时回退到 index.html。
func ServeStatic(r *gin.Engine, distFS http.FileSystem) {
	r.StaticFS("/assets", distFS)
	r.NoRoute(func(c *gin.Context) {
		if c.Request.Method != http.MethodGet {
			c.Status(http.StatusNotFound)
			return
		}
		c.FileFromFS("/", distFS)
	})
}
