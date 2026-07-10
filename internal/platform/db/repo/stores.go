// Package repo 提供数据库持久化层；业务模块仅依赖 Stores 下的领域 Store，不接触 GORM/Transaction。
package repo

import "gorm.io/gorm"

// Stores 是业务层唯一持久化入口，按领域划分功能边界。
type Stores struct {
	Run          *RunStore
	Chat         *ChatStore
	Project      *ProjectStore
	Group        *GroupStore
	Plan         *PlanStore
	Artifact     *ArtifactStore
	Git          *GitStore
	User         *UserStore
	IAM          *IAMStore
	Notification *NotificationStore
	Settings     *SettingsStore
}

// New 创建持久化 Stores。
func New(db *gorm.DB) *Stores {
	c := newCatalog(db)
	return &Stores{
		Run:          newRunStore(c),
		Chat:         newChatStore(c),
		Project:      newProjectStore(c),
		Group:        newGroupStore(c),
		Plan:         newPlanStore(c),
		Artifact:     newArtifactStore(c),
		Git:          newGitStore(c),
		User:         newUserStore(c),
		IAM:          newIAMStore(c),
		Notification: newNotificationStore(c),
		Settings:     newSettingsStore(c),
	}
}
