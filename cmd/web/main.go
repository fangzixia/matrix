// Command web 启动 Matrix HTTP 服务与嵌入式任务 Worker。
package main

import (
	"context"
	"log"
	"matrix/frontend"
	"matrix/internal/app/bootstrap"
	"os"
	"os/signal"
	"syscall"
)

// main 程序入口。
func main() {
	path := bootstrap.ConfigPathFromFlags()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := bootstrap.Run(ctx, bootstrap.Options{ConfigPath: path, StaticFS: frontend.Dist()}); err != nil {
		log.Fatal(err)
	}
}
