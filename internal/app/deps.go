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
	Repositories   *repository.Service
	Groups         *group.Service
	Workspace      *workspace.Service
	WorkspaceRepo  *workspace.ProjectRepoResolver
	RunService     *run.Service
	Notifications  *notification.Service
	Plans          *plan.Service
	Artifacts      *artifact.Service
	SystemSettings *settings.Service
}

// NewDeps 组装应用依赖图（数据库、IAM、Run、通知等）。
func NewDeps(cfg *config.Config, paths storage.Paths, db *gorm.DB, log *slog.Logger, sysSettings *settings.Service) *Deps {
	stores := repo.New(db)
	hub := events.NewHub()
	users := identity.NewUserRepo(stores)
	sessions := identity.NewSessionService(stores, cfg.Auth.Session)
	auth := identity.NewAuthService(users, sessions)
	projects := project.NewService(stores)
	gitRepos := repository.NewService(stores)
	groups := group.NewService(stores)
	ws := workspace.NewService(paths, sysSettings, gitRepos)
	ws.SetProjectKeyResolver(projects)
	resolver := &workspace.ProjectRepoResolver{Projects: projects, Repos: gitRepos, WS: ws}
	notifications := notification.NewService(stores, hub)
	plans := plan.NewService(stores, ws)
	artifactsSvc := artifact.NewService(stores, ws)
	runs := run.NewService(stores, paths, sysSettings, resolver)
	runs.SetNotifier(notifications)
	runs.SetPlans(plans)
	runs.SetArtifacts(artifactsSvc)
	return &Deps{
		Config: cfg, Paths: paths, DB: db, Log: log, Hub: hub,
		Auth: auth, Users: users, Sessions: sessions,
		IAM: iam.NewEnforcer(stores.IAM), Members: iam.NewMemberService(stores.IAM),
		Projects: projects, Repositories: gitRepos, Groups: groups,
		Workspace: ws, WorkspaceRepo: resolver, RunService: runs, Notifications: notifications,
		Plans: plans, Artifacts: artifactsSvc,
		SystemSettings: sysSettings,
	}
}
