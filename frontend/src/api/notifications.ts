/**
 * 站内通知 API。
 */
import { api } from "./client";

/** 用户通知 */
export interface Notification {
  id: string;
  user_id: string;
  kind: string;
  title: string;
  body: string;
  link?: string;
  read_at?: string;
  created_at: string;
}

export function listNotifications() {
  return api<{ notifications: Notification[] }>("/api/notifications");
}

export function unreadCount() {
  return api<{ count: number }>("/api/notifications/unread_count");
}

export function markRead(id: string) {
  return api<{ ok: boolean }>(`/api/notifications/${id}/read`, {
    method: "POST",
  });
}

export function markAllRead() {
  return api<{ ok: boolean }>("/api/notifications/read_all", {
    method: "POST",
  });
}
