/**
 * Admin 用户管理扩展 API：重置密码、封禁/解封。
 */
import { api } from "./client";

/** 重置指定用户密码 */
export function resetUserPassword(userId: string, password: string) {
  return api<{ ok: boolean }>(`/api/admin/users/${userId}/reset_password`, {
    method: "POST",
    body: JSON.stringify({ password }),
  });
}

/** 封禁用户 */
export function blockUser(userId: string) {
  return api<{ ok: boolean }>(`/api/admin/users/${userId}/block`, {
    method: "POST",
  });
}

/** 解封用户 */
export function unblockUser(userId: string) {
  return api<{ ok: boolean }>(`/api/admin/users/${userId}/unblock`, {
    method: "POST",
  });
}
