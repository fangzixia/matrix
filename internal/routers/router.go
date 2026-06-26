// Package routers 注册 Gin 路由并将 HTTP 请求适配到各领域 Service。
package routers

import (
	"io/fs"
	"matrix/internal/app"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// Register 挂载 REST API、Admin 路由与前端静态资源（SPA fallback）。
func Register(r *gin.Engine, d *app.Deps, staticFS fs.FS) {
	// 健康检查，返回服务存活状态
	r.GET("/health", func(c *gin.Context) { c.JSON(200, gin.H{"status": "ok"}) })
	api := r.Group("/api")
	registerAuthRoutes(api, d)
	registerAdminUserRoutes(api, d)
	registerProjectRoutes(api, d)
	registerWorkspaceRoutes(api, d)
	registerRunRoutes(api, d)
	registerChatRoutes(api, d)
	registerMgmtRoutes(api, d)
	registerStaticRoutes(r, staticFS)
}

// registerStaticRoutes 挂载前端 SPA 静态资源与 fallback。
func registerStaticRoutes(r *gin.Engine, staticFS fs.FS) {
	if staticFS == nil {
		return
	}
	// 提供前端静态资源（JS/CSS/SVG 等）
	r.GET("/assets/*filepath", func(c *gin.Context) {
		fp := c.Param("filepath")
		data, err := fs.ReadFile(staticFS, "assets"+fp)
		if err != nil {
			c.Status(404)
			return
		}
		c.Data(200, contentType(fp), data)
	})
	// SPA fallback：非 API GET 请求返回 index.html
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

// contentType 根据静态资源路径返回 HTTP Content-Type。
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
