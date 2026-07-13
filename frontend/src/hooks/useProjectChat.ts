import { useCallback, useEffect, useRef, useState } from "react";
import * as chatApi from "@/api/chat";
import type { ChatMessageNode } from "@/api/chat";
import * as runsApi from "@/api/runs";
import type { AiMessage, ChatAttachment } from "@/components/ai/MatrixAiChat";
import type { RunViewState, StreamMode } from "@/types/runView";
import { startStreamRunViewUntilTerminal } from "@/utils/runViewStreamTask";
import {
  branchThroughNode,
  branchToItems,
  sessionHasContent,
  parentIdForResend,
  parentIdForSend,
  sessionFromApi,
  sessionTitleFromBranch,
} from "@/utils/chatTree";
import { runDebug, runDebugWarn } from "@/utils/runDebug";

export interface ChatSessionState {
  id: string;
  title: string;
  nodes: ChatMessageNode[];
  activeLeafId: string | null;
  updatedAt?: string;
}

function summaryToSession(summary: chatApi.ChatSessionSummary): ChatSessionState {
  return {
    id: summary.id,
    title: summary.title || "新对话",
    nodes: [],
    activeLeafId: summary.active_leaf_id ?? null,
    updatedAt: summary.updated_at,
  };
}

function sessionItems(session: ChatSessionState): AiMessage[] {
  return branchToItems(session.nodes, session.activeLeafId);
}

export function useProjectChat(projectId: string) {
  const stopRef = useRef<(() => void) | null>(null);
  const streamTask = useCallback(
    async (
      pid: string,
      runId: string,
      mode: StreamMode,
      onDelta: (text: string, full: string) => void,
      onViewState?: (state: RunViewState) => void,
    ) => {
      stopRef.current?.();
      const task = startStreamRunViewUntilTerminal(
        pid,
        runId,
        mode,
        onDelta,
        onViewState,
      );
      stopRef.current = task.stop;
      try {
        return await task.promise;
      } finally {
        stopRef.current = null;
      }
    },
    [],
  );
  const stop = useCallback(() => {
    stopRef.current?.();
    stopRef.current = null;
  }, []);
  const activeRunIdRef = useRef<string | null>(null);
  const cancelledRef = useRef(false);

  const [sessions, setSessions] = useState<ChatSessionState[]>([]);
  const [activeSessionId, setActiveSessionId] = useState("");
  const [items, setItems] = useState<AiMessage[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const [booting, setBooting] = useState(true);
  const [capabilities, setCapabilities] = useState<chatApi.ChatCapabilities>({
    model_name: "",
    multimodal: false,
    attachment_types: [],
  });
  const [activityState, setActivityState] = useState<RunViewState | null>(null);
  const [renameOpen, setRenameOpen] = useState(false);
  const [renameTargetId, setRenameTargetId] = useState("");
  const [renameValue, setRenameValue] = useState("");

  const applySession = useCallback((state: ChatSessionState) => {
    setSessions((prev) => {
      const exists = prev.some((s) => s.id === state.id);
      if (!exists) return [state, ...prev];
      return prev.map((s) => (s.id === state.id ? { ...s, ...state } : s));
    });
  }, []);

  const createServerSession = useCallback(async () => {
    const summary = await chatApi.createChatSession(projectId, {
      title: "新对话",
    });
    return summaryToSession(summary);
  }, [projectId]);

  const refreshSession = useCallback(
    async (sid: string) => {
      const remote = await chatApi.getChatSession(projectId, sid);
      const state: ChatSessionState = sessionFromApi(remote);
      applySession(state);
      if (sid === activeSessionId) {
        setItems(sessionItems(state));
      }
      return state;
    },
    [activeSessionId, applySession, projectId],
  );

  const persistSessionMeta = useCallback(
    async (sid: string, patch: Partial<ChatSessionState>) => {
      const session = sessions.find((s) => s.id === sid);
      if (!session) return;
      const next = { ...session, ...patch };
      setSessions((prev) =>
        prev.map((s) => (s.id === sid ? { ...s, ...patch } : s)),
      );
      await chatApi.patchChatSession(projectId, sid, {
        title: next.title,
      });
    },
    [projectId, sessions],
  );

  useEffect(() => {
    let cancelled = false;
    async function load() {
      setBooting(true);
      try {
        const [sessionsRes, caps] = await Promise.all([
          chatApi.listChatSessions(projectId),
          chatApi.getChatCapabilities(projectId),
        ]);
        if (cancelled) return;
        setCapabilities(caps);
        const summaries = sessionsRes.sessions ?? [];
        if (summaries.length) {
          const first = await chatApi.getChatSession(projectId, summaries[0].id);
          if (cancelled) return;
          const firstState: ChatSessionState = sessionFromApi(first);
          const rest: ChatSessionState[] = summaries
            .slice(1)
            .map((s) => summaryToSession(s));
          setSessions([firstState, ...rest]);
          setActiveSessionId(firstState.id);
          setItems(sessionItems(firstState));
        } else {
          const created = await chatApi.createChatSession(projectId, {
            title: "新对话",
          });
          if (cancelled) return;
          const empty = summaryToSession(created);
          setSessions([empty]);
          setActiveSessionId(empty.id);
          setItems([]);
        }
      } catch {
        if (!cancelled) setError("加载对话失败");
      } finally {
        if (!cancelled) setBooting(false);
      }
    }
    load();
    return () => {
      cancelled = true;
    };
  }, [projectId]);

  const switchSession = useCallback(
    async (sid: string) => {
      if (loading || sid === activeSessionId) return;
      const target = sessions.find((s) => s.id === sid);
      if (!target) return;
      setActiveSessionId(sid);
      setError("");
      if (target.nodes.length === 0) {
        try {
          const state = await refreshSession(sid);
          setItems(sessionItems(state));
        } catch (e) {
          setError(e instanceof Error ? e.message : "加载对话失败");
        }
        return;
      }
      setItems(sessionItems(target));
    },
    [activeSessionId, loading, refreshSession, sessions],
  );

  const createNewChat = useCallback(async () => {
    if (loading) return;
    const prevActiveId = activeSessionId;
    const prevActive = sessions.find((s) => s.id === prevActiveId);
    const prevActiveEmpty =
      prevActive != null &&
      items.length === 0 &&
      !sessionHasContent(prevActive.nodes, prevActive.activeLeafId);
    try {
      if (prevActiveEmpty) {
        try {
          await chatApi.deleteChatSession(projectId, prevActiveId);
        } catch {
          /* ignore */
        }
      }
      const created = await createServerSession();
      const { sessions: summaries } = await chatApi.listChatSessions(projectId);
      setSessions((prev) => {
        const byId = new Map(prev.map((s) => [s.id, s]));
        return (summaries ?? []).map((summary) => {
          const existing = byId.get(summary.id);
          if (existing && existing.nodes.length > 0) {
            return {
              ...existing,
              title: summary.title || existing.title,
              activeLeafId: summary.active_leaf_id ?? existing.activeLeafId,
              updatedAt: summary.updated_at,
            };
          }
          return summaryToSession(summary);
        });
      });
      setActiveSessionId(created.id);
      setItems([]);
      setError("");
    } catch {
      setError("创建对话失败");
    }
  }, [
    activeSessionId,
    createServerSession,
    items.length,
    loading,
    projectId,
    sessions,
  ]);

  const openRename = useCallback(
    (sid: string) => {
      const session = sessions.find((s) => s.id === sid);
      if (!session) return;
      setRenameTargetId(sid);
      setRenameValue(session.title);
      setRenameOpen(true);
    },
    [sessions],
  );

  const confirmRename = useCallback(async () => {
    const title = renameValue.trim();
    if (!title || !renameTargetId) return;
    setRenameOpen(false);
    await persistSessionMeta(renameTargetId, { title });
  }, [persistSessionMeta, renameTargetId, renameValue]);

  const deleteSession = useCallback(
    async (sid: string) => {
      const session = sessions.find((s) => s.id === sid);
      if (!session) return;
      await chatApi.deleteChatSession(projectId, sid);
      const next = sessions.filter((s) => s.id !== sid);
      if (sid !== activeSessionId) {
        setSessions(next);
        return;
      }
      if (next.length) {
        const target = next[0];
        setSessions(next);
        setActiveSessionId(target.id);
        if (target.nodes.length === 0) {
          try {
            const state = await refreshSession(target.id);
            setItems(sessionItems(state));
          } catch (e) {
            setError(e instanceof Error ? e.message : "加载对话失败");
          }
        } else {
          setItems(sessionItems(target));
        }
        return;
      }
      try {
        const created = await createServerSession();
        setSessions([created]);
        setActiveSessionId(created.id);
        setItems([]);
      } catch {
        setError("创建对话失败");
        setSessions([]);
        setActiveSessionId("");
        setItems([]);
      }
    },
    [
      activeSessionId,
      createServerSession,
      projectId,
      refreshSession,
      sessions,
    ],
  );

  const handleCancel = useCallback(async () => {
    cancelledRef.current = true;
    const runId = activeRunIdRef.current;
    const sid = activeSessionId;
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
    if (sid) {
      try {
        await refreshSession(sid);
      } catch {
        /* ignore */
      }
    }
  }, [activeSessionId, projectId, refreshSession, stop]);

  const sendInternal = useCallback(
    async (
      text: string,
      attachments: ChatAttachment[] | undefined,
      parentId: string | null,
    ) => {
      if ((!text && !attachments?.length) || !activeSessionId || loading) return;
      setError("");
      setLoading(true);
      setActivityState(null);
      cancelledRef.current = false;
      const sid = activeSessionId;
      const session = sessions.find((s) => s.id === sid);
      const nodes = session?.nodes ?? [];
      const activeLeafId = session?.activeLeafId ?? null;

      const baseItems = parentId
        ? branchThroughNode(nodes, parentId)
        : branchToItems(nodes, activeLeafId);

      const tempUserId = crypto.randomUUID();
      const aiKey = crypto.randomUUID();
      const withUser: AiMessage[] = [
        ...baseItems,
        {
          key: tempUserId,
          role: "user",
          content: text,
          attachments,
          parentId,
        },
        {
          key: aiKey,
          role: "ai",
          content: "",
          loading: true,
          parentId: tempUserId,
        },
      ];
      setItems(withUser);

      let runId = "";
      let serverUserId: string = tempUserId;

      try {
        const run = await chatApi.sendChatMessage(
          projectId,
          sid,
          text,
          attachments,
          parentId,
        );
        runId = run.id;
        serverUserId = run.user_message_id || tempUserId;
        activeRunIdRef.current = run.id;
        runDebug("chat.run.created", {
          runId: run.id,
          sessionId: sid,
          userMessageId: serverUserId,
          status: run.status,
        });
        setItems((prev) =>
          prev.map((item) =>
            item.key === aiKey ? { ...item, runId: run.id } : item,
          ),
        );

        if (serverUserId !== tempUserId) {
          setItems((prev) =>
            prev.map((item) => {
              if (item.key === tempUserId) {
                return { ...item, key: serverUserId };
              }
              if (item.key === aiKey) {
                return { ...item, parentId: serverUserId };
              }
              return item;
            }),
          );
        }

        const reply = await streamTask(
          projectId,
          run.id,
          "chat",
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

        runDebug("chat.run.finished", {
          runId,
          replyLen: reply.length,
          replyEmpty: !reply.trim(),
        });

        const content = reply || "（无回复）";
        const refreshed = await refreshSession(sid);
        runDebug("chat.session.refreshed", {
          sessionId: sid,
          nodeCount: refreshed.nodes.length,
          activeLeafId: refreshed.activeLeafId,
        });
        const title = sessionTitleFromBranch(
          refreshed.nodes,
          refreshed.activeLeafId,
        );
        if (title !== refreshed.title && title !== "新对话") {
          await persistSessionMeta(sid, { title });
        }
        setItems(
          branchToItems(refreshed.nodes, refreshed.activeLeafId).map((item) => {
            if (item.role !== "ai" || item.runId !== runId) return item;
            return { ...item, content: content || item.content };
          }),
        );
      } catch (e) {
        if (cancelledRef.current) {
          try {
            await refreshSession(sid);
          } catch {
            /* ignore */
          }
          return;
        }
        runDebugWarn("chat.send.failed", {
          runId: runId || undefined,
          error: e instanceof Error ? e.message : String(e),
        });
        setItems((prev) => prev.filter((item) => item.key !== aiKey));
        setError(e instanceof Error ? e.message : "发送失败");
      } finally {
        activeRunIdRef.current = null;
        setActivityState(null);
        if (!cancelledRef.current) {
          setLoading(false);
        }
      }
    },
    [
      activeSessionId,
      loading,
      persistSessionMeta,
      projectId,
      refreshSession,
      sessions,
      streamTask,
    ],
  );

  const send = useCallback(
    (message: string, attachments?: ChatAttachment[]) => {
      const session = sessions.find((s) => s.id === activeSessionId);
      const parentId = parentIdForSend(session?.activeLeafId);
      void sendInternal(message.trim(), attachments, parentId);
    },
    [activeSessionId, sendInternal, sessions],
  );

  const resendUserMessage = useCallback(
    (userKey: string | number) => {
      if (loading) return;
      const userMsg = items.find((item) => item.key === userKey);
      if (!userMsg) return;
      const parentId = parentIdForResend(userMsg);
      void sendInternal(userMsg.content, userMsg.attachments, parentId);
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
    booting,
    error,
    loading,
    items,
    sessions,
    activeSessionId,
    capabilities,
    activityState,
    switchSession,
    createNewChat,
    send,
    resendUserMessage,
    toggleMessageActivity,
    handleCancel,
    renameOpen,
    renameValue,
    setRenameValue,
    confirmRename,
    setRenameOpen,
    openRename,
    deleteSession,
  };
}
