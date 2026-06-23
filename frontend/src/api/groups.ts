/**
 * 用户组与组成员 API。
 */
import { api } from "./client";
import type { MemberRole } from "./projects";

/** 当前用户在组中的有效权限 */
export interface GroupPermissions {
  read: boolean;
  manage_members: boolean;
  manage_settings: boolean;
  delete_group: boolean;
}

/** 用户组（用于项目权限继承） */
export interface Group {
  id: string;
  name: string;
  owner_id: string;
  created_at: string;
  updated_at: string;
  current_user_role?: MemberRole;
  permissions?: GroupPermissions;
}

/** 用户组成员 */
export interface GroupMember {
  user_id: string;
  username: string;
  name: string;
  email: string;
  role: MemberRole;
  created_at: string;
}

export function listGroups() {
  return api<{ groups: Group[] }>("/api/groups");
}

export function createGroup(body: { name: string }) {
  return api<Group>("/api/groups", {
    method: "POST",
    body: JSON.stringify(body),
  });
}

export function getGroup(id: string) {
  return api<Group>(`/api/groups/${id}`);
}

export function updateGroup(id: string, body: { name: string }) {
  return api<Group>(`/api/groups/${id}`, {
    method: "PUT",
    body: JSON.stringify(body),
  });
}

export function deleteGroup(id: string) {
  return api<{ ok: boolean }>(`/api/groups/${id}`, { method: "DELETE" });
}

export function listGroupMembers(groupId: string) {
  return api<{ members: GroupMember[] }>(`/api/groups/${groupId}/members`);
}

export function addGroupMember(
  groupId: string,
  body: { user_id: string; role: MemberRole },
) {
  return api<{ ok: boolean }>(`/api/groups/${groupId}/members`, {
    method: "POST",
    body: JSON.stringify(body),
  });
}

export function updateGroupMember(
  groupId: string,
  userId: string,
  role: MemberRole,
) {
  return api<{ ok: boolean }>(`/api/groups/${groupId}/members/${userId}`, {
    method: "PUT",
    body: JSON.stringify({ role }),
  });
}

export function removeGroupMember(groupId: string, userId: string) {
  return api<{ ok: boolean }>(`/api/groups/${groupId}/members/${userId}`, {
    method: "DELETE",
  });
}
