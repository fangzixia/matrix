/**
 * Admin 用户 CRUD 与用户搜索 API。
 */
import { api } from "./client";
import type { User } from "./auth";

/** 带项目数统计的用户（Admin 列表） */
export interface UserWithStats extends User {
  project_count: number;
}

/** 创建用户请求体 */
export interface CreateUserInput {
  username: string;
  email: string;
  password: string;
  name: string;
  is_admin?: boolean;
}

/** 更新用户请求体 */
export interface UpdateUserInput {
  email?: string;
  name?: string;
  is_admin?: boolean;
  state?: string;
  password?: string;
}

/** 按登录名/显示名称/邮箱模糊搜索（添加成员用） */
export function searchUsers(q: string) {
  return api<{ users: User[] }>(`/api/users/search?q=${encodeURIComponent(q)}`);
}

export function listUsers() {
  return api<{ users: UserWithStats[]; total: number }>("/api/admin/users");
}

export function getUser(id: string) {
  return api<User>(`/api/admin/users/${id}`);
}

export function createUser(input: CreateUserInput) {
  return api<User>("/api/admin/users", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export function updateUser(id: string, input: UpdateUserInput) {
  return api<User>(`/api/admin/users/${id}`, {
    method: "PUT",
    body: JSON.stringify(input),
  });
}

export function deleteUser(id: string) {
  return api<{ ok: boolean }>(`/api/admin/users/${id}`, { method: "DELETE" });
}
