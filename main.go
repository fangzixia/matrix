// Matrix Desktop - AI 驱动的软件开发闭环桌面工具
//
// 通过多个专业 Agent 的协作，实现从需求分析到代码实现、再到验收评测的完整开发闭环。
package main

import (
	"log"
	"os"
	"path/filepath"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"

	"matrix/frontend"
	"matrix/internal/desktop"
)

func main() {
	// 初始化日志（写到平台配置目录的 logs/ 子目录）
	if base, err := os.UserConfigDir(); err == nil {
		logDir := filepath.Join(base, "matrix", "logs")
		_ = os.MkdirAll(logDir, 0755)
		logFile := filepath.Join(logDir, "matrix.log")
		if f, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644); err == nil {
			log.SetOutput(f)
		}
	}

	// 加载配置
	cfg, err := desktop.LoadConfig()
	if err != nil {
		log.Printf("Failed to load config: %v", err)
		// 使用默认配置继续
		cfg = desktop.DefaultConfig()
	}

	// 创建 Wails Bridge
	bridge := desktop.NewBridge(cfg)

	// 创建应用
	err = wails.Run(&options.App{
		Title:     "Matrix - AI 开发助手",
		Width:     1280,
		Height:    800,
		MinWidth:  800,
		MinHeight: 600,
		AssetServer: &assetserver.Options{
			Assets: frontend.FS(),
		},
		OnStartup:  bridge.Startup,
		OnShutdown: bridge.Shutdown,
		Bind: []interface{}{
			bridge,
		},
		BackgroundColour: &options.RGBA{R: 15, G: 23, B: 42, A: 1}, // #0F172A
	})

	if err != nil {
		log.Fatalf("Failed to start Matrix application: %v", err)
	}
}
