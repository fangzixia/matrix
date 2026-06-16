import { computed } from 'vue'
import type { Project, MemberRole, ProjectPermissions } from '@/api/projects'

const roleLevel: Record<MemberRole, number> = {
  guest: 10,
  reporter: 20,
  developer: 30,
  maintainer: 40,
  owner: 50,
}

export function useProjectPermissions(project: () => Project | null) {
  const permissions = computed(() => project()?.permissions ?? null)
  const role = computed(() => project()?.current_user_role ?? null)

  function hasPermission(key: keyof ProjectPermissions): boolean {
    return !!permissions.value?.[key]
  }

  function atLeast(min: MemberRole): boolean {
    const r = role.value
    if (!r) {
      return min === 'guest' && !!permissions.value?.read
    }
    return roleLevel[r] >= roleLevel[min]
  }

  return {
    permissions,
    role,
    hasPermission,
    atLeast,
    canManageSettings: computed(() => hasPermission('manage_settings')),
    canManageMembers: computed(() => hasPermission('manage_members')),
    canDeleteProject: computed(() => hasPermission('delete_project')),
    canCreateRun: computed(() => hasPermission('create_run')),
  }
}
