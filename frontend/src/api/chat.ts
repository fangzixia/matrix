/**
 * Chat 会话 API。
 */
import { api } from "./client";
import type { Run } from "./runs";

export interface ChatAttachment {
  type: string;
  mime_type: string;
  name: string;
  data: string;
}

export interface ChatMessageNode {
  id: string;
  parent_id: string | null;
  role: "user" | "assistant" | "system";
  content: string;
  attachments?: ChatAttachment[];
  run_id?: string;
  created_at?: string;
}

export interface ChatSessionSummary {
  id: string;
  title: string;
  active_leaf_id?: string | null;
  updated_at?: string;
}

export interface ChatSession extends ChatSessionSummary {
  project_id?: string;
  nodes?: ChatMessageNode[];
}

export interface ChatCapabilities {
  model_name: string;
  multimodal: boolean;
  attachment_types: string[];
}

export interface ChatRun extends Run {
  user_message_id?: string;
  assistant_message_id?: string;
}

export function getChatCapabilities(projectId: string) {
  return api<ChatCapabilities>(
    `/api/projects/${projectId}/chat/capabilities`,
  );
}

export function listChatSessions(projectId: string) {
  return api<{ sessions: ChatSessionSummary[] }>(
    `/api/projects/${projectId}/chat/sessions`,
  );
}

export function getChatSession(projectId: string, sessionId: string) {
  return api<ChatSession>(
    `/api/projects/${projectId}/chat/sessions/${sessionId}`,
  );
}

export interface CreateChatSessionPayload {
  id?: string;
  title?: string;
}

export function createChatSession(
  projectId: string,
  payload: CreateChatSessionPayload,
) {
  return api<ChatSessionSummary>(
    `/api/projects/${projectId}/chat/sessions`,
    {
      method: "POST",
      body: JSON.stringify(payload),
    },
  );
}

export interface PatchChatSessionPayload {
  title?: string;
}

export function patchChatSession(
  projectId: string,
  sessionId: string,
  payload: PatchChatSessionPayload,
) {
  return api<ChatSessionSummary>(
    `/api/projects/${projectId}/chat/sessions/${sessionId}`,
    {
      method: "PATCH",
      body: JSON.stringify(payload),
    },
  );
}

export function deleteChatSession(projectId: string, sessionId: string) {
  return api<{ ok: boolean }>(
    `/api/projects/${projectId}/chat/sessions/${sessionId}`,
    { method: "DELETE" },
  );
}

export function sendChatMessage(
  projectId: string,
  sessionId: string,
  message: string,
  attachments?: ChatAttachment[],
  parentId?: string | null,
) {
  return api<ChatRun>(`/api/projects/${projectId}/chat/sessions/${sessionId}/run`, {
    method: "POST",
    body: JSON.stringify({
      message,
      parent_id: parentId ?? null,
      attachments: attachments?.length ? attachments : undefined,
    }),
  });
}
