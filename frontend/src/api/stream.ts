/**
 * 用户通知 SSE 流订阅。
 */
import type { Notification } from "./notifications";

/** 通知 SSE 推送载荷 */
export interface NotificationStreamPayload {
  type?: string;
  notification: Notification;
}

/**
 * subscribeNotificationStream 订阅用户通知 SSE，返回取消订阅函数。
 */
export function subscribeNotificationStream(
  onMessage: (payload: NotificationStreamPayload) => void,
): () => void {
  const es = new EventSource("/api/notifications/stream", {
    withCredentials: true,
  });
  es.addEventListener("notification", (ev) => {
    try {
      onMessage(
        JSON.parse((ev as MessageEvent).data) as NotificationStreamPayload,
      );
    } catch {
      /* ignore malformed payloads */
    }
  });
  es.onerror = () => {
    es.close();
  };
  return () => es.close();
}
