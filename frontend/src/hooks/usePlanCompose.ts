import { useCallback, useRef, useState } from "react";
import * as runsApi from "@/api/runs";
import type { AiMessage } from "@/components/ai/MatrixAiChat";
import { useRunViewStream } from "@/hooks/useRunViewStream";
import type { RunViewState } from "@/types/runView";
import { runDebug, runDebugWarn } from "@/utils/runDebug";

export function usePlanCompose(
  projectId: string,
  filePath: string,
  onRunComplete?: () => void,
) {
  const { streamTask, stop } = useRunViewStream();
  const activeRunIdRef = useRef<string | null>(null);
  const cancelledRef = useRef(false);
  const filePathRef = useRef(filePath);
  filePathRef.current = filePath;

  const [items, setItems] = useState<AiMessage[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const [activityState, setActivityState] = useState<RunViewState | null>(null);

  const handleCancel = useCallback(async () => {
    cancelledRef.current = true;
    const runId = activeRunIdRef.current;
    stop();
    if (runId) {
      try {
        await runsApi.cancelRun(projectId, runId);
      } catch {
        /* ignore */
      }
    }
    setLoading(false);
    setActivityState(null);
    setItems((prev) =>
      prev.map((item) => {
        if (!item.loading) return item;
        const content = item.content.trim()
          ? `${item.content}\n\n（已停止）`
          : "（已停止）";
        return { ...item, content, loading: false };
      }),
    );
  }, [projectId, stop]);

  const sendInternal = useCallback(
    async (text: string) => {
      const trimmed = text.trim();
      if (!trimmed || loading) return;

      setError("");
      setLoading(true);
      setActivityState(null);
      cancelledRef.current = false;

      const userKey = crypto.randomUUID();
      const aiKey = crypto.randomUUID();

      setItems((prev) => [
        ...prev,
        { key: userKey, role: "user", content: trimmed },
        {
          key: aiKey,
          role: "ai",
          content: "",
          loading: true,
          parentId: userKey,
        },
      ]);

      let runId = "";

      try {
        const run = await runsApi.startRun(
          projectId,
          trimmed,
          "plan",
          filePathRef.current,
        );
        runId = run.id;
        activeRunIdRef.current = run.id;
        runDebug("plan.run.created", { runId: run.id, status: run.status });

        setItems((prev) =>
          prev.map((item) =>
            item.key === aiKey ? { ...item, runId: run.id } : item,
          ),
        );

        const reply = await streamTask(
          projectId,
          run.id,
          "detail",
          (_delta, full) => {
            if (cancelledRef.current) return;
            setItems((prev) =>
              prev.map((item) =>
                item.key === aiKey
                  ? { ...item, content: full, loading: true }
                  : item,
              ),
            );
          },
          (state) => {
            setActivityState(state);
          },
        );
        if (cancelledRef.current) return;

        runDebug("plan.run.finished", {
          runId,
          replyLen: reply.length,
          replyEmpty: !reply.trim(),
        });

        const content = reply || "（无回复）";
        setItems((prev) =>
          prev.map((item) =>
            item.key === aiKey
              ? { ...item, content, loading: false }
              : item,
          ),
        );
        onRunComplete?.();
      } catch (e) {
        if (cancelledRef.current) return;
        runDebugWarn("plan.send.failed", {
          runId: runId || undefined,
          error: e instanceof Error ? e.message : String(e),
        });
        setItems((prev) => prev.filter((item) => item.key !== aiKey));
        setError(e instanceof Error ? e.message : "发送失败");
      } finally {
        activeRunIdRef.current = null;
        if (!cancelledRef.current) {
          setLoading(false);
        }
      }
    },
    [loading, onRunComplete, projectId, streamTask],
  );

  const send = useCallback(
    (message: string) => {
      void sendInternal(message);
    },
    [sendInternal],
  );

  const resendUserMessage = useCallback(
    (userKey: string | number) => {
      if (loading) return;
      const userMsg = items.find((item) => item.key === userKey);
      if (!userMsg) return;
      void sendInternal(userMsg.content);
    },
    [items, loading, sendInternal],
  );

  const toggleMessageActivity = useCallback((key: string | number) => {
    setItems((prev) =>
      prev.map((item) =>
        item.key === key
          ? { ...item, activityExpanded: !item.activityExpanded }
          : item,
      ),
    );
  }, []);

  return {
    error,
    loading,
    items,
    activityState,
    send,
    resendUserMessage,
    toggleMessageActivity,
    handleCancel,
  };
}
