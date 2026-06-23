import { useMemo } from "react";
import type { Group, GroupPermissions } from "@/api/groups";
import type { MemberRole } from "@/api/projects";

const roleLevel: Record<MemberRole, number> = {
  guest: 10,
  reporter: 20,
  developer: 30,
  maintainer: 40,
  owner: 50,
};

export function useGroupPermissions(group: Group | null) {
  const permissions = group?.permissions ?? null;
  const role = group?.current_user_role ?? null;
  return useMemo(() => {
    function hasPermission(key: keyof GroupPermissions): boolean {
      return !!permissions?.[key];
    }
    function atLeast(min: MemberRole): boolean {
      if (!role) {
        return min === "guest" && !!permissions?.read;
      }
      return roleLevel[role] >= roleLevel[min];
    }
    return {
      permissions,
      role,
      hasPermission,
      atLeast,
      canManageMembers: hasPermission("manage_members"),
      canManageSettings: hasPermission("manage_settings"),
      canDeleteGroup: hasPermission("delete_group"),
    };
  }, [permissions, role]);
}
