package repo

import "gorm.io/gorm"

// catalog 聚合内部实体 Repo，仅供 Store 使用，不暴露给业务层。
type catalog struct {
	run               *RunRepo
	runStep           *RunStepRepo
	runView           *RunViewRepo
	chatSession       *ChatSessionRepo
	chatMessage       *ChatMessageRepo
	project           *ProjectRepo
	projectMember     *ProjectMemberRepo
	projectRepository *ProjectRepositoryRepo
	group             *GroupRepo
	groupMember       *GroupMemberRepo
	user              *UserRepo
	session           *SessionRepo
	notification      *NotificationRepo
	plan              *PlanRepo
	planResolution    *PlanResolutionRepo
	artifact          *ArtifactRepo
	systemSetting     *SystemSettingRepo
}

func newCatalog(db *gorm.DB) *catalog {
	return &catalog{
		run:               NewRunRepo(db),
		runStep:           NewRunStepRepo(db),
		runView:           NewRunViewRepo(db),
		chatSession:       NewChatSessionRepo(db),
		chatMessage:       NewChatMessageRepo(db),
		project:           NewProjectRepo(db),
		projectMember:     NewProjectMemberRepo(db),
		projectRepository: NewProjectRepositoryRepo(db),
		group:             NewGroupRepo(db),
		groupMember:       NewGroupMemberRepo(db),
		user:              NewUserRepo(db),
		session:           NewSessionRepo(db),
		notification:      NewNotificationRepo(db),
		plan:              NewPlanRepo(db),
		planResolution:    NewPlanResolutionRepo(db),
		artifact:          NewArtifactRepo(db),
		systemSetting:     NewSystemSettingRepo(db),
	}
}
