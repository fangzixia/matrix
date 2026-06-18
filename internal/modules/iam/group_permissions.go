package iam

// GroupPermissions 描述当前用户在组中的有效权限。
type GroupPermissions struct {
	Read           bool `json:"read"`
	ManageMembers  bool `json:"manage_members"`
	ManageSettings bool `json:"manage_settings"`
	DeleteGroup    bool `json:"delete_group"`
}

// PermissionsForGroupRole 返回角色对应的组权限集。
func PermissionsForGroupRole(role Role) GroupPermissions {
	switch {
	case RoleAtLeast(role, RoleOwner):
		return GroupPermissions{
			Read: true, ManageMembers: true, ManageSettings: true, DeleteGroup: true,
		}
	case RoleAtLeast(role, RoleMaintainer):
		return GroupPermissions{
			Read: true, ManageMembers: true, ManageSettings: true, DeleteGroup: false,
		}
	default:
		return GroupPermissions{Read: true}
	}
}
