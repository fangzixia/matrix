/**
 * 项目、成员与仓库文件 API。
 */
import { api } from "./client";

/** 项目可见性 */
export type ProjectVisibility = "private" | "internal" | "public";

/** 项目成员角色（GitLab 风格五级） */
export type MemberRole =
  | "guest"
  | "reporter"
  | "developer"
  | "maintainer"
  | "owner";

/** 当前用户在项目中的有效权限 */
export interface ProjectPermissions {
  read: boolean;
  create_run: boolean;
  manage_settings: boolean;
  manage_members: boolean;
  delete_project: boolean;
  git_pull: boolean;
  git_push: boolean;
}

/** 项目详情 DTO */
export interface Project {
  id: string;
  name: string;
  path?: string;
  git_url: string;
  git_branch: string;
  visibility: ProjectVisibility;
  group_id?: string | null;
  owner_id: string;
  created_at: string;
  updated_at: string;
  current_user_role?: MemberRole | null;
  permissions?: ProjectPermissions;
}

/** 项目成员列表项 */
export interface ProjectMember {
  user_id: string;
  username: string;
  name: string;
  email: string;
  role: MemberRole;
  created_at: string;
}

/** 仓库文件树节点 */
export interface FileEntry {
  name: string;
  path: string;
  is_dir: boolean;
  size?: number;
}

/** 计划文档摘要 */
export interface PlanItem {
  id?: string;
  path: string;
  title: string;
  status?: "draft" | "approved" | string;
  content?: string;
  run_id?: string;
}

/** 评测/产物摘要 */
export interface EvaluationItem {
  id?: string;
  kind?: string;
  path: string;
  plan_path?: string;
  title: string;
  content?: string;
  run_id?: string;
}

export function listProjects(scope: "yours" | "explore" | "starred" = "yours") {
  return api<{ projects: Project[] }>(`/api/projects?scope=${scope}`);
}

export function getProject(id: string) {
  return api<Project>(`/api/projects/${id}`);
}

export function createProject(body: {
  name: string;
  path: string;
  git_url?: string;
  git_branch?: string;
  visibility?: ProjectVisibility;
}) {
  return api<Project>("/api/projects", {
    method: "POST",
    body: JSON.stringify(body),
  });
}

export function updateProject(
  id: string,
  body: {
    name?: string;
    path?: string;
    git_url?: string;
    git_branch?: string;
    visibility?: ProjectVisibility;
    group_id?: string | null;
  },
) {
  return api<Project>(`/api/projects/${id}`, {
    method: "PUT",
    body: JSON.stringify(body),
  });
}

export function deleteProject(id: string) {
  return api<{ ok: boolean }>(`/api/projects/${id}`, { method: "DELETE" });
}

export function listMembers(projectId: string) {
  return api<{ members: ProjectMember[] }>(
    `/api/projects/${projectId}/members`,
  );
}

export function addMember(
  projectId: string,
  body: { username?: string; user_id?: string; role: MemberRole },
) {
  return api<{ ok: boolean }>(`/api/projects/${projectId}/members`, {
    method: "POST",
    body: JSON.stringify(body),
  });
}

export function updateMember(
  projectId: string,
  userId: string,
  role: MemberRole,
) {
  return api<{ ok: boolean }>(`/api/projects/${projectId}/members/${userId}`, {
    method: "PUT",
    body: JSON.stringify({ role }),
  });
}

export function removeMember(projectId: string, userId: string) {
  return api<{ ok: boolean }>(`/api/projects/${projectId}/members/${userId}`, {
    method: "DELETE",
  });
}

export function listFiles(projectId: string, runId: string, path = "") {
  const params = new URLSearchParams({ run_id: runId });
  if (path) params.set("path", path);
  return api<{ files: FileEntry[] }>(
    `/api/projects/${projectId}/repository/tree?${params}`,
  );
}

export function readFile(projectId: string, runId: string, path: string) {
  const params = new URLSearchParams({
    run_id: runId,
    path,
  });
  return api<{ content: string }>(
    `/api/projects/${projectId}/repository/file?${params}`,
  );
}

export function listPlans(projectId: string) {
  return api<{ plans: PlanItem[] | null }>(`/api/projects/${projectId}/plans`);
}

export function approvePlan(
  projectId: string,
  body: { path: string; resolutions: Record<string, string> },
) {
  return api<{ ok: boolean; status: string }>(
    `/api/projects/${projectId}/plans/approve`,
    {
      method: "POST",
      body: JSON.stringify(body),
    },
  );
}

export function listEvaluations(projectId: string) {
  return api<{ evaluations: EvaluationItem[] | null }>(
    `/api/projects/${projectId}/evaluations`,
  );
}

/** 将 ISO 时间格式化为中文相对时间（如「3 小时前」） */
export function formatRelativeTime(iso: string): string {
  const diff = Date.now() - new Date(iso).getTime();
  const mins = Math.floor(diff / 60000);
  if (mins < 60) return `${mins || 1} 分钟前`;
  const hours = Math.floor(mins / 60);
  if (hours < 24) return `${hours} 小时前`;
  const days = Math.floor(hours / 24);
  if (days < 30) return `${days} 天前`;
  return new Date(iso).toLocaleDateString("zh-CN");
}

/** 角色中文标签 */
export const roleLabels: Record<MemberRole, string> = {
  guest: "访客",
  reporter: "报告者",
  developer: "开发者",
  maintainer: "维护者",
  owner: "所有者",
};

/** 成员角色 Select 选项 */
export const memberRoleOptions: {
  value: MemberRole;
  label: string;
  hint: string;
}[] = [
  {
    value: "guest",
    label: roleLabels.guest,
    hint: "只读访问项目、仓库与 Runs",
  },
  { value: "reporter", label: roleLabels.reporter, hint: "访客 + Git 拉取" },
  {
    value: "developer",
    label: roleLabels.developer,
    hint: "创建 Run、对话写入、Git 拉取",
  },
  {
    value: "maintainer",
    label: roleLabels.maintainer,
    hint: "管理设置与成员、Git 推送",
  },
  { value: "owner", label: roleLabels.owner, hint: "完全控制，含删除项目" },
];

/** 项目可见性标签 */
export const visibilityLabels: Record<ProjectVisibility, string> = {
  private: "私有",
  internal: "内部",
  public: "公开",
};

export const visibilityTitles: Record<ProjectVisibility, string> = {
  private: "私有 — 仅项目成员可访问",
  internal: "内部 — 所有已登录用户可访问",
  public: "公开 — 所有已登录用户可访问",
};
