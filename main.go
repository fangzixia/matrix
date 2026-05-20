// Matrix Desktop - AI 驱动的软件开发闭环桌面工具
//
// 通过多个专业 Agent 的协作，实现从需求分析到代码实现、再到验收评测的完整开发闭环。
package main

import (
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"

	"matrix/frontend"
	"matrix/internal/desktop"
	"matrix/internal/logger"
)

func main() {
	if err := logger.Init(); err != nil {
		logger.Errorf("Failed to initialize logger: %v", err)
	}

	cfg, err := desktop.LoadConfig()
	if err != nil {
		logger.Warnf("Failed to load config: %v", err)
		cfg = desktop.DefaultConfig()
	}

	bridge := desktop.NewBridge(cfg)

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
		logger.Fatalf("Failed to start Matrix application: %v", err)
	}
}
