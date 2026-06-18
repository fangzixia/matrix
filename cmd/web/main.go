// Command web 启动 Matrix HTTP 服务与嵌入式任务 Worker。
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"matrix/frontend"
	"matrix/internal/app/bootstrap"
)

func main() {
	path := bootstrap.ConfigPathFromFlags()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := bootstrap.Run(ctx, bootstrap.Options{ConfigPath: path, StaticFS: frontend.Dist()}); err != nil {
		log.Fatal(err)
	}
}
