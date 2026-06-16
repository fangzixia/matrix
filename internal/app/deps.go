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
	"matrix/internal/modules/requirement"
	"matrix/internal/modules/run"
	"matrix/internal/modules/systemsettings"
	"matrix/internal/modules/workspace"
	"matrix/internal/platform/config"
	"matrix/internal/platform/events"
	"matrix/internal/platform/storage"
)

type Deps struct {
	Config         *config.Config
	Paths          storage.Paths
	DB             *gorm.DB
	Log            *slog.Logger
	Hub            *events.Hub
	Auth           *identity.AuthService
	Users          *identity.UserRepo
	Sessions       *identity.SessionService
	IAM            *iam.Enforcer
	Members        *iam.MemberService
	Projects       *project.Service
	Settings       *project.SettingsService
	Repositories   *repository.Service
	Groups         *group.Service
	Workspace      *workspace.Service
	Runs           *run.Service
	Jobs           *job.Service
	Pipeline       *pipeline.Service
	Notifications  *notification.Service
	Requirements   *requirement.Service
	Artifacts      *artifact.Service
	Runtime        *run.Runtime
	SystemSettings *systemsettings.Service
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

	runs := run.NewService(db, rt, hub, paths, cfg, resolver, settings)
	runs.SetJobEnqueuer(jobs)
	runs.SetNotifier(notifications)
	runs.SetPipeline(pipe)
	runs.SetPullAll(ws.PullAll)

	return &Deps{
		Config: cfg, Paths: paths, DB: db, Log: log, Hub: hub,
		Auth: auth, Users: users, Sessions: sessions,
		IAM: iam.NewEnforcer(db), Members: iam.NewMemberService(db),
		Projects: projects, Settings: settings, Repositories: repos, Groups: groups,
		Workspace: ws, Runs: runs, Jobs: jobs, Pipeline: pipe, Notifications: notifications,
		Requirements: requirement.NewService(db, ws),
		Artifacts:    artifact.NewService(db, ws),
		Runtime:      rt, SystemSettings: sysSettings,
	}
}

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
