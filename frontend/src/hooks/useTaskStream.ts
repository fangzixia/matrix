import { useCallback, useRef } from "react";
import { subscribeRunStream } from "@/api/stream";
import * as runsApi from "@/api/runs";
import type { RunEvent } from "@/api/runs";
import type { StreamMessage } from "@/types/runStream";
import {
  buildRunActivityState,
  extractReplyText,
} from "@/utils/runActivityParser";

/** 从 SSE 载荷中提取可追加的文本片段（单条消息）。 */
export function extractStreamText(data: unknown): string | null {
  if (!data || typeof data !== "object") return null;
  const msg = data as StreamMessage;
  if (
    msg.type === "stream_event" &&
    msg.event?.type === "content_block_delta"
  ) {
    const delta = msg.event.delta;
    if (delta?.type === "text_delta" && delta.text) return delta.text;
  }
  if (msg.type === "assistant" && msg.message?.content) {
    const text = msg.message.content
      .filter((b) => b.type === "text" && b.text)
      .map((b) => b.text)
      .join("");
    if (text) return text;
  }
  if (msg.type === "result" && msg.output) return msg.output;
  return null;
}

function extractAssistantFromEvents(events: RunEvent[]): string {
  return extractReplyText(
    buildRunActivityState(events, [] as StreamMessage[]),
  );
}

/**
 * 订阅任务 SSE，并轮询持久化事件以补全流式输出（避免 SSE 晚于 Run 启动而丢增量）。
 */
export function useTaskStream() {
  const unsubscribeRef = useRef<(() => void) | null>(null);
  const stop = useCallback(() => {
    unsubscribeRef.current?.();
    unsubscribeRef.current = null;
  }, []);
  const streamTask = useCallback(
    async (
      projectId: string,
      taskId: string,
      onDelta: (text: string, full: string) => void,
    ): Promise<string> => {
      stop();
      const liveMessages: StreamMessage[] = [];
      const seenUuids = new Set<string>();
      let events: RunEvent[] = [];
      let afterId: string | undefined;
      let lastFull = "";

      function pushLive(data: unknown) {
        if (!data || typeof data !== "object") return;
        const msg = data as StreamMessage;
        if (msg.uuid) {
          if (seenUuids.has(msg.uuid)) return;
          seenUuids.add(msg.uuid);
        }
        liveMessages.push(msg);
      }

      function sync() {
        const full = extractReplyText(
          buildRunActivityState(events, liveMessages),
        );
        if (full !== lastFull) {
          const delta = full.startsWith(lastFull)
            ? full.slice(lastFull.length)
            : full;
          lastFull = full;
          onDelta(delta, full);
        }
        return full;
      }

      unsubscribeRef.current = subscribeRunStream(
        projectId,
        taskId,
        (data) => {
          pushLive(data);
          sync();
        },
      );

      for (;;) {
        const batch = await runsApi.listRunEvents(projectId, taskId, afterId);
        if (batch.events.length) {
          events = events.concat(batch.events);
          afterId = batch.events.at(-1)?.id;
          sync();
        }
        const run = await runsApi.getRun(projectId, taskId);
        if (!["running", "queued", "pending"].includes(run.status)) {
          const tail = await runsApi.listRunEvents(projectId, taskId, afterId);
          if (tail.events.length) {
            events = events.concat(tail.events);
            sync();
          }
          stop();
          let full = lastFull.trim();
          if (!full) {
            full = extractAssistantFromEvents(events).trim();
          }
          if (!full) {
            try {
              const audit = await runsApi.getRunAudit(projectId, taskId);
              if (audit.content?.trim()) full = audit.content.trim();
            } catch {
              /* ignore */
            }
          }
          if (!full && run.status === "failed" && run.error_message?.trim()) {
            return run.error_message.trim();
          }
          return full;
        }
        await new Promise((r) => setTimeout(r, 300));
      }
    },
    [stop],
  );
  return { streamTask, stop };
}
