// Package bootstrap Web 服务启动链：配置、迁移、依赖注入、HTTP 监听。
package bootstrap

import (
	"context"
	"flag"
	"fmt"
	"io/fs"
	"matrix/internal/app"
	"matrix/internal/modules/identity"
	"matrix/internal/modules/settings"
	"matrix/internal/platform/config"
	platformdb "matrix/internal/platform/db"
	platformhttp "matrix/internal/platform/http"
	"matrix/internal/platform/logging"
	"matrix/internal/platform/migrate"
	"matrix/internal/platform/storage"
	"matrix/internal/routers"
	"os"

	"github.com/gin-gonic/gin"
)

// Options 是 Web 服务启动时的配置选项。
type Options struct {
	ConfigPath string
	StaticFS   fs.FS
}

// Run 启动 Web 服务：加载配置、迁移数据库、注入依赖并监听 HTTP。
func Run(ctx context.Context, opts Options) error {
	cfg, err := config.Load(opts.ConfigPath)
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}
	paths, err := storage.Resolve(cfg)
	if err != nil {
		return err
	}
	if err := storage.EnsureLayout(paths); err != nil {
		return err
	}
	dev := cfg.System.Env == "development"
	log, err := logging.Init(cfg.Logging, paths, dev)
	if err != nil {
		return err
	}
	log.Info("存储路径已解析", "data_dir", paths.DataDir, "log_dir", paths.LogDir)
	db, err := platformdb.Open(cfg.Database)
	if err != nil {
		return err
	}
	if err := migrate.Up(db, cfg.Database); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	if err := identity.BootstrapAdmin(ctx, db, cfg.Auth); err != nil {
		return err
	}
	runtime := config.DefaultRuntime()
	sysSettings := settings.NewService(db, runtime)
	if err := sysSettings.Bootstrap(ctx); err != nil {
		return fmt.Errorf("system settings: %w", err)
	}
	deps := app.NewDeps(cfg, runtime, paths, db, log, sysSettings)
	if err := deps.Repositories.MigrateLegacyProjects(ctx); err != nil {
		log.Warn("迁移旧版仓库绑定失败", "err", err)
	}
	deps.Runs.SetLifecycle(ctx) // 进程退出时取消进行中的 Run
	defer deps.Close()
	deps.StartJobWorker(ctx) // 嵌入式任务队列消费者
	engine := platformhttp.NewEngine(log, dev)
	routers.Register(engine, deps, opts.StaticFS)
	log.Info("HTTP 服务监听中", "addr", cfg.Server.Addr)
	srv := &httpServer{engine: engine, addr: cfg.Server.Addr}
	return srv.ListenAndServe()
}

type httpServer struct {
	engine *gin.Engine
	addr   string
}

// ListenAndServe 在配置的地址上启动 Gin HTTP 服务。
func (s *httpServer) ListenAndServe() error {
	return s.engine.Run(s.addr)
}

// ConfigPathFromFlags 解析 -config 命令行参数，默认 config/config.yml（可用 MATRIX_CONFIG 覆盖）。
func ConfigPathFromFlags() string {
	path := flag.String("config", envOr("MATRIX_CONFIG", "config/config.yml"), "config file path")
	flag.Parse()
	return *path
}

// envOr 读取环境变量，为空时返回默认值。
func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
