/**
 * Run 视图 SSE 与 REST API。
 */
import { api } from "./client";
import type { LoggedEvent, RunViewState, StreamMode } from "@/types/runView";
import { normalizeRunViewState } from "@/utils/viewReducer";
import { runDebug, runDebugWarn } from "@/utils/runDebug";

export interface RunViewResponse {
  state: RunViewState | null;
}

export interface ToolLogResponse {
  content: string;
}

export interface RunViewStreamHandlers {
  onEvent: (logged: LoggedEvent) => void;
  onDisconnect?: () => void;
}

export interface RunViewStreamOptions {
  afterSeq?: number;
}

/** 获取 Run 活动视图快照。 */
export async function getRunView(projectId: string, runId: string) {
  const res = await api<RunViewResponse>(
    `/api/projects/${projectId}/runs/${runId}/view`,
  );
  return {
    ...res,
    state: res.state ? normalizeRunViewState(res.state) : null,
  };
}

/** 获取工具 spill 日志。 */
export function getToolLog(
  projectId: string,
  runId: string,
  toolUseId: string,
) {
  return api<ToolLogResponse>(
    `/api/projects/${projectId}/runs/${runId}/tools/${toolUseId}/log`,
  );
}

/** 订阅 Run 视图 SSE（服务端 DB 轮询推送），返回取消函数。 */
export function subscribeRunViewStream(
  projectId: string,
  runId: string,
  mode: StreamMode,
  handlers: RunViewStreamHandlers,
  options: RunViewStreamOptions = {},
): () => void {
  const params = new URLSearchParams({ mode });
  if (options.afterSeq && options.afterSeq > 0) {
    params.set("afterSeq", String(options.afterSeq));
  }
  const url = `/api/projects/${projectId}/runs/${runId}/stream?${params.toString()}`;
  runDebug("sse.connect", { runId, projectId, mode, url });
  const es = new EventSource(url, { withCredentials: true });

  es.addEventListener("run:view", (ev) => {
    try {
      const logged = JSON.parse(
        (ev as MessageEvent).data,
      ) as LoggedEvent;
      handlers.onEvent(logged);
    } catch (e) {
      runDebugWarn("sse.parse_error", {
        runId,
        error: e instanceof Error ? e.message : String(e),
      });
    }
  });

  es.onopen = () => {
    runDebug("sse.open", { runId });
  };

  es.onerror = () => {
    runDebugWarn("sse.error", { runId, readyState: es.readyState });
    handlers.onDisconnect?.();
  };

  return () => {
    runDebug("sse.close", { runId });
    es.close();
  };
}

/** 非对话页 Run 状态轮询间隔（毫秒）。 */
export function runStatusPollIntervalMs() {
  return 10_000;
}
