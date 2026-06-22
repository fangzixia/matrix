// Package app 组装应用依赖图，供 webapp 与 bootstrap 注入使用。
package app

import (
	"context"
	"log/slog"

	"gorm.io/gorm"

	"matrix/internal/app/worker"
	"matrix/internal/modules/artifact"
	"matrix/internal/modules/group"
	"matrix/internal/modules/iam"
	"matrix/internal/modules/identity"
	"matrix/internal/modules/job"
	"matrix/internal/modules/notification"
	"matrix/internal/modules/pipeline"
	"matrix/internal/modules/project"
	"matrix/internal/modules/repository"
	"matrix/internal/modules/plan"
	"matrix/internal/modules/run"
	"matrix/internal/modules/systemsettings"
	"matrix/internal/modules/workspace"
	"matrix/internal/platform/config"
	"matrix/internal/platform/events"
	"matrix/internal/platform/storage"
)

// Deps 应用级依赖容器：各领域 Service 与共享基础设施。
type Deps struct {
	Config         *config.Config           // YAML + 热更新后的运行配置
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
	Settings       *project.SettingsService // 项目集成设置
	Repositories   *repository.Service      // Git 仓库绑定
	Groups         *group.Service           // 用户组
	Workspace      *workspace.Service       // Git 工作区
	Runs           *run.Service             // AI Run/Chat
	Jobs           *job.Service             // 任务队列
	Pipeline       *pipeline.Service        // 流水线阶段
	Notifications  *notification.Service    // 通知
	Plans          *plan.Service            // 计划文档
	Artifacts      *artifact.Service        // 评测产物
	Runtime        *run.Runtime             // AI Agent 运行时
	SystemSettings *systemsettings.Service  // 系统级配置（DB）
}

// NewDeps 组装应用依赖图（数据库、IAM、Run 队列、通知等）。
func NewDeps(cfg *config.Config, paths storage.Paths, db *gorm.DB, log *slog.Logger) *Deps {
	hub := events.NewHub()
	users := identity.NewUserRepo(db)
	sessions := identity.NewSessionService(db, cfg.Auth.Session)
	auth := identity.NewAuthService(users, sessions)
	projects := project.NewService(db)
	settings := project.NewSettingsService(db)
	repos := repository.NewService(db)
	groups := group.NewService(db)
	ws := workspace.NewService(paths, cfg.Git, repos)
	resolver := &workspace.ProjectRepoResolver{Projects: projects, Repos: repos, WS: ws}
	rt := run.NewRuntime(cfg)
	pipe := pipeline.NewService(cfg.Pipeline)
	sysSettings := systemsettings.NewService(db, cfg)
	sysSettings.SetHooks(systemsettings.Hooks{
		OnGitUpdate:      ws.UpdateGit,
		OnPipelineUpdate: pipe.UpdateConfig,
	})
	jobs := job.NewService(db, cfg.Worker.MaxAttempts)
	notifications := notification.NewService(db, hub)

	plans := plan.NewService(db, ws)
	artifactsSvc := artifact.NewService(db, ws)

	runs := run.NewService(db, rt, hub, paths, cfg, resolver, settings)
	runs.SetJobEnqueuer(jobs)
	runs.SetNotifier(notifications)
	runs.SetPipeline(pipe)
	runs.SetPullAll(ws.PullAll)
	runs.SetPlans(plans)
	runs.SetArtifacts(artifactsSvc)

	return &Deps{
		Config: cfg, Paths: paths, DB: db, Log: log, Hub: hub,
		Auth: auth, Users: users, Sessions: sessions,
		IAM: iam.NewEnforcer(db), Members: iam.NewMemberService(db),
		Projects: projects, Settings: settings, Repositories: repos, Groups: groups,
		Workspace: ws, Runs: runs, Jobs: jobs, Pipeline: pipe, Notifications: notifications,
		Plans: plans, Artifacts: artifactsSvc,
		Runtime:      rt, SystemSettings: sysSettings,
	}
}

// Close 释放应用依赖持有的资源（如 AI 运行时）。
func (d *Deps) Close() {
	if d.Runtime != nil {
		d.Runtime.Close()
	}
}

// StartJobWorker 在 Web 进程内启动嵌入式任务消费者（与 HTTP 服务同进程）。
func (d *Deps) StartJobWorker(ctx context.Context) {
	if d.Jobs == nil || d.Runs == nil || !d.Config.Worker.Enabled {
		return
	}
	wid := worker.ID()
	d.Log.Info("embedded job worker starting",
		"worker_id", wid,
		"concurrency", d.Config.Worker.Concurrency,
		"poll_interval", d.Config.Worker.PollInterval,
	)
	go d.Jobs.RunWorker(ctx, wid, d.Config.Worker.PollInterval, d.Config.Worker.Concurrency, d.Runs)
}
