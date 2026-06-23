// Package app 组装应用依赖图，供 routers 与 bootstrap 注入使用。
package app

import (
	"context"
	"log/slog"
	"matrix/internal/app/worker"
	"matrix/internal/modules/artifact"
	"matrix/internal/modules/group"
	"matrix/internal/modules/iam"
	"matrix/internal/modules/identity"
	"matrix/internal/modules/job"
	"matrix/internal/modules/notification"
	"matrix/internal/modules/pipeline"
	"matrix/internal/modules/plan"
	"matrix/internal/modules/project"
	"matrix/internal/modules/repository"
	"matrix/internal/modules/run"
	"matrix/internal/modules/settings"
	"matrix/internal/modules/workspace"
	"matrix/internal/platform/config"
	"matrix/internal/platform/events"
	"matrix/internal/platform/storage"

	"gorm.io/gorm"
)

// Deps 应用级依赖容器：各领域 Service 与共享基础设施。
type Deps struct {
	Config         *config.Config           // config.yml 文件配置
	Runtime        *config.RuntimeConfig    // 运行时配置（DB 热更新）
	Paths          storage.Paths            // 数据/工作区/审计目录
	DB             *gorm.DB                 // PostgreSQL
	Log            *slog.Logger             // 结构化日志
	Hub            *events.Hub              // SSE 事件总线
	Auth           *identity.AuthService    // 登录校验
	Users          *identity.UserRepo       // 用户 CRUD
	Sessions       *identity.SessionService // Session Cookie
	IAM            *iam.Enforcer            // 项目 RBAC
	Members        *iam.MemberService       // 项目成员
	Projects       *project.Service         // 项目
	Repositories   *repository.Service      // Git 仓库绑定
	Groups         *group.Service           // 用户组
	Workspace      *workspace.Service       // Git 工作区
	Runs           *run.Service             // AI Run/Chat
	Jobs           *job.Service             // 任务队列
	Pipeline       *pipeline.Service        // 流水线阶段
	Notifications  *notification.Service    // 通知
	Plans          *plan.Service            // 计划文档
	Artifacts      *artifact.Service        // 评测产物
	RunRuntime     *run.Runtime             // AI Agent 运行时
	SystemSettings *settings.Service        // 系统级配置（DB）
}

// NewDeps 组装应用依赖图（数据库、IAM、Run 队列、通知等）。
// runtime 须已在 SystemSettings.Bootstrap 中从数据库加载。
func NewDeps(cfg *config.Config, runtime *config.RuntimeConfig, paths storage.Paths, db *gorm.DB, log *slog.Logger, sysSettings *settings.Service) *Deps {
	hub := events.NewHub()
	users := identity.NewUserRepo(db)
	sessions := identity.NewSessionService(db, cfg.Auth.Session)
	auth := identity.NewAuthService(users, sessions)
	projects := project.NewService(db)
	repos := repository.NewService(db)
	groups := group.NewService(db)
	ws := workspace.NewService(paths, runtime.Git, repos)
	ws.SetProjectKeyResolver(projects)
	resolver := &workspace.ProjectRepoResolver{Projects: projects, Repos: repos, WS: ws}
	rt := run.NewRuntime(runtime)
	pipe := pipeline.NewService(runtime.Pipeline)
	sysSettings.SetHooks(settings.Hooks{
		OnGitUpdate:      ws.UpdateGit,
		OnPipelineUpdate: pipe.UpdateConfig,
	})
	jobs := job.NewService(db, runtime.Worker.MaxAttempts)
	notifications := notification.NewService(db, hub)
	plans := plan.NewService(db, ws)
	artifactsSvc := artifact.NewService(db, ws)
	runs := run.NewService(db, rt, hub, paths, runtime, resolver)
	runs.SetJobEnqueuer(jobs)
	runs.SetNotifier(notifications)
	runs.SetPipeline(pipe)
	runs.SetPullAll(ws.PullAll)
	runs.SetPlans(plans)
	runs.SetArtifacts(artifactsSvc)
	return &Deps{
		Config: cfg, Runtime: runtime, Paths: paths, DB: db, Log: log, Hub: hub,
		Auth: auth, Users: users, Sessions: sessions,
		IAM: iam.NewEnforcer(db), Members: iam.NewMemberService(db),
		Projects: projects, Repositories: repos, Groups: groups,
		Workspace: ws, Runs: runs, Jobs: jobs, Pipeline: pipe, Notifications: notifications,
		Plans: plans, Artifacts: artifactsSvc,
		RunRuntime: rt, SystemSettings: sysSettings,
	}
}

// Close 释放应用依赖持有的资源（如 AI 运行时）。
func (d *Deps) Close() {
	if d.RunRuntime != nil {
		d.RunRuntime.Close()
	}
}

// StartJobWorker 在 Web 进程内启动嵌入式任务消费者（与 HTTP 服务同进程）。
func (d *Deps) StartJobWorker(ctx context.Context) {
	if d.Jobs == nil || d.Runs == nil || !d.Runtime.Worker.Enabled {
		return
	}
	wid := worker.ID()
	d.Log.Info("embedded job worker starting",
		"worker_id", wid,
		"concurrency", d.Runtime.Worker.Concurrency,
		"poll_interval", d.Runtime.Worker.PollInterval,
	)
	go d.Jobs.RunWorker(ctx, wid, d.Runtime.Worker.PollInterval, d.Runtime.Worker.Concurrency, d.Runs)
}
