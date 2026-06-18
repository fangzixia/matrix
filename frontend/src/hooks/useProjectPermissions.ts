import { useMemo } from 'react'
import type { Project, MemberRole, ProjectPermissions } from '@/api/projects'

const roleLevel: Record<MemberRole, number> = {
  guest: 10,
  reporter: 20,
  developer: 30,
  maintainer: 40,
  owner: 50,
}

export function useProjectPermissions(project: Project | null) {
  const permissions = project?.permissions ?? null
  const role = project?.current_user_role ?? null

  return useMemo(() => {
    function hasPermission(key: keyof ProjectPermissions): boolean {
      return !!permissions?.[key]
    }

    function atLeast(min: MemberRole): boolean {
      if (!role) {
        return min === 'guest' && !!permissions?.read
      }
      return roleLevel[role] >= roleLevel[min]
    }

    return {
      permissions,
      role,
      hasPermission,
      atLeast,
      canManageSettings: hasPermission('manage_settings'),
      canManageMembers: hasPermission('manage_members'),
      canDeleteProject: hasPermission('delete_project'),
      canCreateRun: hasPermission('create_run'),
    }
  }, [permissions, role])
}
