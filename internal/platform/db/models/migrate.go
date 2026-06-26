package models

// All 返回 GORM AutoMigrate 应注册的全部模型指针。
func All() []any {
	return []any{
		&User{}, &Session{}, &Group{}, &GroupMember{},
		&Project{}, &ProjectMember{}, &ProjectRepository{},
		&Run{}, &RunStep{}, &RunView{}, &RunJob{},
		&Notification{}, &ChatSession{}, &ChatMessage{}, &PlanResolution{},
		&SystemSetting{},
		&Plan{}, &Artifact{},
	}
}
