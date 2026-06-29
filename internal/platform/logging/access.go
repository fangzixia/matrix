package logging

import (
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// AccessLog 写入 nginx combined 风格的 API 访问日志。
type AccessLog struct {
	w  io.Writer
	mu sync.Mutex
}

// WriteCombined 记录一条 HTTP 访问日志。
func (a *AccessLog) WriteCombined(c *gin.Context, latency time.Duration) {
	if a == nil || a.w == nil || c == nil || c.Request == nil {
		return
	}
	uri := c.Request.URL.RequestURI()
	if uri == "" {
		uri = c.Request.URL.Path
	}
	proto := c.Request.Proto
	if proto == "" {
		proto = "HTTP/1.1"
	}
	referer := c.Request.Referer()
	userAgent := c.Request.UserAgent()
	line := fmt.Sprintf(
		`%s - - [%s] "%s %s %s" %d %d "%s" "%s" rt=%dms request_id=%s`,
		c.ClientIP(),
		time.Now().Format("02/Jan/2006:15:04:05 -0700"),
		c.Request.Method,
		uri,
		proto,
		c.Writer.Status(),
		c.Writer.Size(),
		referer,
		userAgent,
		latency.Milliseconds(),
		c.GetString("request_id"),
	)
	a.mu.Lock()
	defer a.mu.Unlock()
	if _, err := a.w.Write(append([]byte(line), '\n')); err != nil {
		logWriteFallback("logging: API access 行写入失败", err)
	}
}
