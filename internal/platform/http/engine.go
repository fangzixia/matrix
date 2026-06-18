// Package http 封装 Gin 引擎、请求日志与统一 JSON 错误响应。
package http

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ErrorBody 是 API 统一错误响应 JSON 结构。
type ErrorBody struct {
	Error   string `json:"error"`
	Message string `json:"message"`
	Code    string `json:"code,omitempty"`
}

// NewEngine 创建带 Recovery、Request-ID 与访问日志的 Gin 引擎。
func NewEngine(log *slog.Logger, dev bool) *gin.Engine {
	if !dev {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(requestIDMiddleware())
	r.Use(slogMiddleware(log))
	return r
}

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

func slogMiddleware(log *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		log.Info("http",
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"latency_ms", time.Since(start).Milliseconds(),
			"request_id", c.GetString("request_id"),
		)
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
