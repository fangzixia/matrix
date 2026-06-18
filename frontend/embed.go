// Package frontend 嵌入 Vite 构建的前端静态资源，供 Web 服务托管。
package frontend

import (
	"embed"
	"io/fs"
)

// 构建 dist 后再编译后端：go generate ./frontend/... && go build ./cmd/web
// 也可直接运行 scripts/build-web.bat（Windows）或 scripts/build-web.sh。
//
//go:generate go run ../tools/buildfrontend

//go:embed dist/*
var dist embed.FS

// Dist 返回嵌入的前端 dist 子目录文件系统。
func Dist() fs.FS {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		return dist
	}
	return sub
}
