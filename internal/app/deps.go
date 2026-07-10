// Package app 组装应用依赖图，供 routers 与 bootstrap 注入使用。
package app

import (
	"log/slog"
	"matrix/internal/modules/artifact"
	"matrix/internal/modules/group"
	"matrix/internal/modules/iam"
	"matrix/internal/modules/identity"
	"matrix/internal/modules/notification"
	"matrix/internal/modules/plan"
	"matrix/internal/modules/project"
	"matrix/internal/modules/repository"
	"matrix/internal/modules/run"
	"matrix/internal/modules/settings"
	"matrix/internal/modules/workspace"
	"matrix/internal/platform/config"
	"matrix/internal/platform/db/repo"
	"matrix/internal/platform/events"
	"matrix/internal/platform/storage"

	"gorm.io/gorm"
)

// Deps 应用级依赖容器：各领域 Service 与共享基础设施。
type Deps struct {
	Config         *config.Config                 // config.yml 文件配置
	Runtime        *config.RuntimeConfig          // 运行时配置（DB 热更新）
	Paths          storage.Paths                  // 数据/工作区/审计目录
	DB             *gorm.DB                       // PostgreSQL
	Log            *slog.Logger                   // 结构化日志
	Hub            *events.Hub                    // SSE 事件总线
	Auth           *identity.AuthService          // 登录校验
	Users          *identity.UserRepo             // 用户 CRUD
	Sessions       *identity.SessionService       // Session Cookie
	IAM            *iam.Enforcer                  // 项目 RBAC
	Members        *iam.MemberService             // 项目成员
	Projects       *project.Service               // 项目
	Repositories   *repository.Service            // Git 仓库绑定
	Groups         *group.Service                 // 用户组
	Workspace      *workspace.Service             // 工作区（Run 级仓库与文档）
	WorkspaceRepo  *workspace.ProjectRepoResolver // Run 沙箱解析
	RunService     *run.Service                   // AI Run/Chat
	Notifications  *notification.Service          // 通知
	Plans          *plan.Service                  // 计划文档
	Artifacts      *artifact.Service              // 评测产物
	RunRuntime     *run.Runtime                   // AI Agent 运行时
	SystemSettings *settings.Service              // 系统级配置（DB）
}

// NewDeps 组装应用依赖图（数据库、IAM、Run、通知等）。
// runtime 须已在 SystemSettings.Bootstrap 中从数据库加载。
func NewDeps(cfg *config.Config, runtime *config.RuntimeConfig, paths storage.Paths, db *gorm.DB, log *slog.Logger, sysSettings *settings.Service) *Deps {
	stores := repo.New(db)
	hub := events.NewHub()
	users := identity.NewUserRepo(stores)
	sessions := identity.NewSessionService(stores, cfg.Auth.Session)
	auth := identity.NewAuthService(users, sessions)
	projects := project.NewService(stores)
	gitRepos := repository.NewService(stores)
	groups := group.NewService(stores)
	ws := workspace.NewService(paths, runtime.Git, gitRepos)
	ws.SetProjectKeyResolver(projects)
	resolver := &workspace.ProjectRepoResolver{Projects: projects, Repos: gitRepos, WS: ws}
	rt := run.NewRuntime(runtime)
	sysSettings.SetHooks(settings.Hooks{
		OnGitUpdate: ws.UpdateGit,
	})
	notifications := notification.NewService(stores, hub)
	plans := plan.NewService(stores, ws)
	artifactsSvc := artifact.NewService(stores, ws)
	runs := run.NewService(stores, rt, hub, paths, runtime, resolver)
	runs.SetNotifier(notifications)
	runs.SetPlans(plans)
	runs.SetArtifacts(artifactsSvc)
	runs.SetAIRuntimeReloader(sysSettings)
	return &Deps{
		Config: cfg, Runtime: runtime, Paths: paths, DB: db, Log: log, Hub: hub,
		Auth: auth, Users: users, Sessions: sessions,
		IAM: iam.NewEnforcer(stores.IAM), Members: iam.NewMemberService(stores.IAM),
		Projects: projects, Repositories: gitRepos, Groups: groups,
		Workspace: ws, WorkspaceRepo: resolver, RunService: runs, Notifications: notifications,
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
