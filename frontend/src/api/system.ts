/**
 * 系统配置 API：root 用户在 /admin/system 各 Tab 独立读写。
 * 对应后端 /api/admin/system/settings/{ai|mcp|git|worker|pipeline}
 */
import { api } from "./client";

/** LLM 模型配置项 */
export interface ModelProfile {
  id: string;
  name: string;
  base_url: string;
  api_key?: string;
  api_key_set?: boolean;
  model: string;
  max_tokens: number;
  enabled: boolean;
  default: boolean;
}

/** 对话上下文压缩参数 */
export interface SystemContextSettings {
  auto_compact_threshold: number;
  keep_recent_messages: number;
}

/** Agent 沙箱安全策略 */
export interface SystemSecuritySettings {
  allow_shell: boolean;
  allow_command_mcp: boolean;
  shell_timeout: string;
}

/** AI 域：模型列表 + 上下文 + 安全 */
export interface AISettings {
  models: ModelProfile[];
  context: SystemContextSettings;
  security: SystemSecuritySettings;
}

/** 单个 MCP 服务器连接参数 */
export interface SystemMCPServer {
  command?: string;
  args?: string[];
  url?: string;
  disabled?: boolean;
  headers?: Record<string, string>;
  env?: Record<string, string>;
}

/** MCP 域配置 */
export interface MCPSettings {
  mcp_servers: Record<string, SystemMCPServer>;
}

/** Git SSH 访问规则 */
export interface GitAccess {
  id: string;
  name: string;
  host: string;
  ssh_key_path: string;
}

/** Git 域配置 */
export interface SystemGitSettings {
  clone_timeout: string;
  ssh_key_path?: string;
  accesses: GitAccess[];
  platform?: string;
  platform_label?: string;
  default_ssh_key_path?: string;
}

/** Worker 任务队列参数 */
export interface SystemWorkerSettings {
  enabled: boolean;
  poll_interval: string;
  max_attempts: number;
  concurrency: number;
}

/** 流水线默认阶段 */
export interface SystemPipelineSettings {
  default_stages: string[];
  pull_before_stage: boolean;
}

export function getAISettings() {
  return api<AISettings>("/api/admin/system/settings/ai");
}

export function saveAISettings(body: AISettings) {
  return api<AISettings>("/api/admin/system/settings/ai", {
    method: "PUT",
    body: JSON.stringify(body),
  });
}

export function getMcpSettings() {
  return api<MCPSettings>("/api/admin/system/settings/mcp");
}

export function saveMcpSettings(mcp_servers: Record<string, SystemMCPServer>) {
  return api<MCPSettings>("/api/admin/system/settings/mcp", {
    method: "PUT",
    body: JSON.stringify({ mcp_servers }),
  });
}

export function getGitSettings() {
  return api<SystemGitSettings>("/api/admin/system/settings/git");
}

export function saveGitSettings(body: SystemGitSettings) {
  return api<SystemGitSettings>("/api/admin/system/settings/git", {
    method: "PUT",
    body: JSON.stringify(body),
  });
}

export function testGitAccess(git_url: string) {
  return api<{ ok: boolean; message: string }>(
    "/api/admin/system/settings/git/test",
    {
      method: "POST",
      body: JSON.stringify({ git_url }),
    },
  );
}

export function getWorkerSettings() {
  return api<SystemWorkerSettings>("/api/admin/system/settings/worker");
}

export function saveWorkerSettings(body: SystemWorkerSettings) {
  return api<SystemWorkerSettings>("/api/admin/system/settings/worker", {
    method: "PUT",
    body: JSON.stringify(body),
  });
}

export function getPipelineSettings() {
  return api<SystemPipelineSettings>("/api/admin/system/settings/pipeline");
}

export function savePipelineSettings(body: SystemPipelineSettings) {
  return api<SystemPipelineSettings>("/api/admin/system/settings/pipeline", {
    method: "PUT",
    body: JSON.stringify(body),
  });
}
