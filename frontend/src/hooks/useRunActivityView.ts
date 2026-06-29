import { useCallback, useEffect, useRef, useState } from "react";
import {
  getRunView,
  subscribeRunViewStream,
  type RunViewStreamHandlers,
} from "@/api/runView";
import type { RunViewState, StreamMode, ViewEnvelope } from "@/types/runView";
import { applyEnvelope } from "@/utils/viewReducer";
import { runDebugWarn } from "@/utils/runDebug";

const TERMINAL_STATUSES = new Set(["succeeded", "failed", "cancelled"]);

export interface UseRunActivityViewOptions {
  live?: boolean;
  mode?: StreamMode;
  onTerminal?: (state: RunViewState) => void;
}

export function isRunViewTerminal(state: RunViewState | null): boolean {
  return Boolean(state && TERMINAL_STATUSES.has(state.status));
}

/**
 * 统一加载和订阅 Run 活动视图。
 *
 * 后端以 DB 快照流为权威；这里按 seq 去重，并把最后 seq 传给 SSE
 * catch-up，避免断线后从头重放。
 */
export function useRunActivityView(
  projectId: string | undefined,
  runId: string | undefined,
  options: UseRunActivityViewOptions = {},
) {
  const { live = false, mode = "detail", onTerminal } = options;
  const [state, setState] = useState<RunViewState | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const [disconnected, setDisconnected] = useState(false);
  const lastSeqRef = useRef(0);
  const terminalNotifiedRef = useRef(false);
  const unsubscribeRef = useRef<(() => void) | null>(null);

  const reset = useCallback(() => {
    lastSeqRef.current = 0;
    terminalNotifiedRef.current = false;
    setState(null);
    setError("");
    setDisconnected(false);
  }, []);

  const notifyTerminal = useCallback(
    (next: RunViewState) => {
      if (!isRunViewTerminal(next) || terminalNotifiedRef.current) return;
      terminalNotifiedRef.current = true;
      onTerminal?.(next);
    },
    [onTerminal],
  );

  const applyViewEnvelope = useCallback(
    (env: ViewEnvelope) => {
      if (env.seq > 0 && env.seq <= lastSeqRef.current) return;
      if (env.seq > 0) lastSeqRef.current = env.seq;
      setDisconnected(false);
      setState((prev) => {
        const next = applyEnvelope(prev, env);
        notifyTerminal(next);
        return next;
      });
    },
    [notifyTerminal],
  );

  const reload = useCallback(async () => {
    if (!projectId || !runId) {
      reset();
      return null;
    }
    setLoading(true);
    setError("");
    try {
      const res = await getRunView(projectId, runId);
      const next = res.state;
      setState(next);
      lastSeqRef.current = next?.seq ?? 0;
      if (next) notifyTerminal(next);
      return next;
    } catch (e) {
      const message = e instanceof Error ? e.message : "加载失败";
      setError(message);
      return null;
    } finally {
      setLoading(false);
    }
  }, [notifyTerminal, projectId, reset, runId]);

  const stop = useCallback(() => {
    unsubscribeRef.current?.();
    unsubscribeRef.current = null;
  }, []);

  useEffect(() => {
    reset();
    if (!projectId || !runId) return;
    const currentProjectId = projectId;
    const currentRunId = runId;
    let cancelled = false;

    async function start() {
      const initial = await reload();
      if (cancelled || !live) return;
      const handlers: RunViewStreamHandlers = {
        onEnvelope: applyViewEnvelope,
        onDisconnect: () => {
          runDebugWarn("activity_view.sse.disconnect", { runId: currentRunId });
          setDisconnected(true);
          void reload();
        },
      };
      unsubscribeRef.current = subscribeRunViewStream(
        currentProjectId,
        currentRunId,
        mode,
        handlers,
        { afterSeq: initial?.seq ?? lastSeqRef.current },
      );
    }

    void start();
    return () => {
      cancelled = true;
      stop();
    };
  }, [applyViewEnvelope, live, mode, projectId, reload, reset, runId, stop]);

  return {
    state,
    loading,
    error,
    disconnected,
    lastSeq: lastSeqRef.current,
    reload,
    stop,
  };
}
