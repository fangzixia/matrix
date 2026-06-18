package iam

// ProjectPermissions 描述当前用户在项目中的有效权限（GitLab 风格角色映射）。
type ProjectPermissions struct {
	Read           bool `json:"read"`
	CreateRun      bool `json:"create_run"`
	ManageSettings bool `json:"manage_settings"`
	ManageMembers  bool `json:"manage_members"`
	DeleteProject  bool `json:"delete_project"`
	GitPull        bool `json:"git_pull"`
	GitPush        bool `json:"git_push"`
}

// PermissionsForRole 返回角色对应的 GitLab 风格项目权限集。
func PermissionsForRole(role Role) ProjectPermissions {
	switch {
	case RoleAtLeast(role, RoleOwner):
		return ProjectPermissions{
			Read: true, CreateRun: true, ManageSettings: true,
			ManageMembers: true, DeleteProject: true, GitPull: true, GitPush: true,
		}
	case RoleAtLeast(role, RoleMaintainer):
		return ProjectPermissions{
			Read: true, CreateRun: true, ManageSettings: true,
			ManageMembers: true, DeleteProject: false, GitPull: true, GitPush: true,
		}
	case RoleAtLeast(role, RoleDeveloper):
		return ProjectPermissions{
			Read: true, CreateRun: true, ManageSettings: false,
			ManageMembers: false, DeleteProject: false, GitPull: true, GitPush: false,
		}
	case RoleAtLeast(role, RoleReporter):
		return ProjectPermissions{
			Read: true, CreateRun: false, ManageSettings: false,
			ManageMembers: false, DeleteProject: false, GitPull: true, GitPush: false,
		}
	default:
		return ProjectPermissions{Read: true}
	}
}

// GuestPermissions 非成员访问 internal/public 项目时的只读权限。
func GuestPermissions() ProjectPermissions {
	return ProjectPermissions{Read: true}
}
