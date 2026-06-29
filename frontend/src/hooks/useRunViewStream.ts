import { useCallback, useRef } from "react";
import {
  getRunView,
  subscribeRunViewStream,
  type RunViewStreamHandlers,
} from "@/api/runView";
import * as runsApi from "@/api/runs";
import type { RunViewState, StreamMode, ViewEnvelope } from "@/types/runView";
import {
  applyEnvelope,
  extractReplyText,
  extractRunFailure,
  formatUserRunError,
} from "@/utils/viewReducer";
import { runDebug, runDebugWarn } from "@/utils/runDebug";

interface RunFinishedState {
  status: string;
  output?: string;
  error_message?: string;
}

const TERMINAL_STATUSES = new Set(["succeeded", "failed", "cancelled"]);
const RUN_POLL_MS = 3_000;

function envelopeKey(env: ViewEnvelope): string {
  if (env.seq > 0) return `${env.type}:${env.seq}`;
  return `${env.type}:${env.timestamp}`;
}

function isTerminalStatus(status: string): boolean {
  return TERMINAL_STATUSES.has(status);
}

/**
 * 订阅 Run 视图 SSE；结束后等待 RUN_FINISHED，断线或超时时 getRun 兜底。
 */
export function useRunViewStream() {
  const unsubscribeRef = useRef<(() => void) | null>(null);
  const pollTimerRef = useRef<number | null>(null);
  const resolveTerminalRef = useRef<
    ((payload: RunFinishedState | null) => void) | null
  >(null);

  const stop = useCallback(() => {
    if (pollTimerRef.current != null) {
      window.clearInterval(pollTimerRef.current);
      pollTimerRef.current = null;
    }
    unsubscribeRef.current?.();
    unsubscribeRef.current = null;
    if (resolveTerminalRef.current) {
      runDebugWarn("stream.stop.before_terminal", {});
      resolveTerminalRef.current(null);
    }
    resolveTerminalRef.current = null;
  }, []);

  const streamTask = useCallback(
    async (
      projectId: string,
      taskId: string,
      mode: StreamMode,
      onDelta: (text: string, full: string) => void,
      onViewState?: (state: RunViewState) => void,
    ): Promise<string> => {
      stop();
      let state: RunViewState | null = null;
      let lastFull = "";
      const seenKeys = new Set<string>();
      let lastSeq = 0;
      let terminalReceived = false;

      function resolveTerminal(payload: RunFinishedState, source: string) {
        if (terminalReceived) return;
        terminalReceived = true;
        runDebug("stream.terminal", {
          runId: taskId,
          source,
          status: payload.status,
          outputLen: payload.output?.length ?? 0,
          error: payload.error_message,
        });
        resolveTerminalRef.current?.(payload);
        resolveTerminalRef.current = null;
      }

      function handleEnvelope(env: ViewEnvelope) {
        const key = envelopeKey(env);
        if (seenKeys.has(key)) return;
        seenKeys.add(key);
        if (env.seq > lastSeq) lastSeq = env.seq;
        runDebug("stream.envelope", {
          runId: taskId,
          type: env.type,
          seq: env.seq,
        });
        if (env.type === "RUN_FINISHED") {
          const pl = env.payload as {
            status: string;
            output?: string;
            error?: string;
          };
          resolveTerminal(
            {
              status: pl.status,
              output: pl.output,
              error_message: pl.error,
            },
            "sse",
          );
        }
        state = applyEnvelope(state, env);
        const full = extractReplyText(state);
        if (full !== lastFull) {
          const delta = full.startsWith(lastFull)
            ? full.slice(lastFull.length)
            : full;
          lastFull = full;
          onDelta(delta, full);
        }
        if (state) onViewState?.(state);
      }

      const terminalPromise = new Promise<RunFinishedState | null>((resolve) => {
        resolveTerminalRef.current = resolve;
      });

      const pollTerminal = async () => {
        if (terminalReceived || resolveTerminalRef.current == null) return;
        try {
          const run = await runsApi.getRun(projectId, taskId);
          runDebug("stream.poll", {
            runId: taskId,
            status: run.status,
            outputLen: run.output?.length ?? 0,
          });
          if (!isTerminalStatus(run.status)) return;
          resolveTerminal(
            {
              status: run.status,
              output: run.output,
              error_message: run.error_message,
            },
            "poll",
          );
        } catch (e) {
          runDebugWarn("stream.poll.error", {
            runId: taskId,
            error: e instanceof Error ? e.message : String(e),
          });
        }
      };

      runDebug("stream.start", { runId: taskId, projectId, mode });

      const handlers: RunViewStreamHandlers = {
        onEnvelope: handleEnvelope,
        onDisconnect: () => {
          runDebugWarn("stream.sse.disconnect", { runId: taskId });
          if (terminalReceived) return;
          void pollTerminal();
        },
      };

      unsubscribeRef.current = subscribeRunViewStream(
        projectId,
        taskId,
        mode,
        handlers,
        { afterSeq: lastSeq },
      );
      pollTimerRef.current = window.setInterval(() => {
        void pollTerminal();
      }, RUN_POLL_MS);

      const terminal = await terminalPromise;
      stop();

      let finalState: RunFinishedState;
      if (terminal) {
        finalState = {
          status: terminal.status,
          output: terminal.output,
          error_message: terminal.error_message,
        };
      } else {
        runDebugWarn("stream.terminal.null", { runId: taskId });
        const run = await runsApi.getRun(projectId, taskId);
        finalState = {
          status: run.status,
          output: run.output,
          error_message: run.error_message,
        };
        if (!state) {
          const viewRes = await getRunView(projectId, taskId);
          if (viewRes.state) {
            state = viewRes.state;
            onViewState?.(viewRes.state);
          }
        }
      }

      runDebug("stream.done", {
        runId: taskId,
        status: finalState.status,
        outputLen: finalState.output?.length ?? 0,
        streamedLen: lastFull.length,
        hasViewState: state != null,
      });

      const failure = extractRunFailure(state, finalState.error_message);
      if (finalState.status === "failed") {
        throw new Error(
          failure ??
            formatUserRunError(finalState.error_message || "任务执行失败"),
        );
      }
      if (finalState.status === "cancelled") {
        throw new Error("任务已取消");
      }
      if (failure) {
        throw new Error(failure);
      }

      const output = finalState.output?.trim();
      if (output) return output;
      const streamed =
        lastFull.trim() || extractReplyText(state).trim();
      if (streamed) return streamed;
      if (finalState.status === "succeeded") {
        runDebugWarn("stream.empty_reply", {
          runId: taskId,
          status: finalState.status,
        });
      }
      return "";
    },
    [stop],
  );

  const loadView = useCallback(async (projectId: string, runId: string) => {
    const res = await getRunView(projectId, runId);
    return res.state;
  }, []);

  return { streamTask, loadView, stop };
}
