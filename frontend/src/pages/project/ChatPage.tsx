import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useParams } from "react-router-dom";
import {
  Alert,
  Flex,
  Input,
  Modal,
  Select,
  Spin,
  Typography,
  theme,
} from "antd";
import type { MenuProps } from "antd";
import {
  DeleteOutlined,
  EditOutlined,
  MessageOutlined,
} from "@ant-design/icons";
import { Conversations } from "@ant-design/x";
import type { ConversationItemType, ConversationsProps } from "@ant-design/x";
import MatrixAiChat, {
  type AiMessage,
  type ChatAttachment,
  type ChatPromptItem,
} from "@/components/ai/MatrixAiChat";
import RunActivityPanel from "@/components/ai/RunActivityPanel";
import * as chatApi from "@/api/chat";
import type { ChatMessageNode } from "@/api/chat";
import * as runsApi from "@/api/runs";
import { useRunViewStream } from "@/hooks/useRunViewStream";
import type { RunViewState } from "@/types/runView";
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

interface SessionState {
  id: string;
  title: string;
  modelId?: string;
  nodes: ChatMessageNode[];
  activeLeafId: string | null;
  updatedAt?: string;
}

function summaryToSession(
  summary: chatApi.ChatSessionSummary,
  defaultModelId?: string,
): SessionState {
  return {
    id: summary.id,
    title: summary.title || "新对话",
    modelId: summary.model_id || defaultModelId,
    nodes: [],
    activeLeafId: summary.active_leaf_id ?? null,
    updatedAt: summary.updated_at,
  };
}

const DEFAULT_PROMPTS: ChatPromptItem[] = [
  {
    key: "summary",
    label: "总结当前项目文档",
    description: "快速了解项目现状与关键文档",
  },
  {
    key: "plan",
    label: "帮我写实现计划",
    description: "基于需求拆解可执行的开发步骤",
  },
  {
    key: "review",
    label: "审查代码潜在问题",
    description: "从可维护性与风险角度给出建议",
  },
  {
    key: "explain",
    label: "解释项目架构",
    description: "梳理模块职责与数据流向",
  },
];

function sessionItems(session: SessionState): AiMessage[] {
  return branchToItems(session.nodes, session.activeLeafId);
}

export default function ChatPage() {
  const { id: projectId = "" } = useParams();
  const { token } = theme.useToken();
  const headerHeight = token.Layout?.headerHeight ?? 48;
  const { streamTask, stop } = useRunViewStream();
  const activeRunIdRef = useRef<string | null>(null);
  const cancelledRef = useRef(false);

  const [sessions, setSessions] = useState<SessionState[]>([]);
  const [activeSessionId, setActiveSessionId] = useState("");
  const [items, setItems] = useState<AiMessage[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const [booting, setBooting] = useState(true);
  const [capabilities, setCapabilities] = useState<chatApi.ChatCapabilities>({
    model_name: "",
    multimodal: false,
    attachment_types: [],
    models: [],
  });
  const [activityState, setActivityState] = useState<RunViewState | null>(
    null,
  );
  const [renameOpen, setRenameOpen] = useState(false);
  const [renameTargetId, setRenameTargetId] = useState("");
  const [renameValue, setRenameValue] = useState("");

  const activeSession = useMemo(
    () => sessions.find((s) => s.id === activeSessionId),
    [sessions, activeSessionId],
  );

  const sessionModelId =
    activeSession?.modelId || capabilities.default_model_id;

  const sessionModelCaps = useMemo(
    () => chatApi.modelCapabilities(capabilities, sessionModelId),
    [capabilities, sessionModelId],
  );

  const modelOptions = useMemo(
    () =>
      (capabilities.models ?? []).map((m) => ({
        id: m.id,
        name: m.name,
      })),
    [capabilities.models],
  );

  const applySession = useCallback((state: SessionState) => {
    setSessions((prev) => {
      const exists = prev.some((s) => s.id === state.id);
      if (!exists) return [state, ...prev];
      return prev.map((s) => (s.id === state.id ? { ...s, ...state } : s));
    });
  }, []);

  const createServerSession = useCallback(
    async (modelId?: string) => {
      const summary = await chatApi.createChatSession(projectId, {
        title: "新对话",
        model_id: modelId,
      });
      return summaryToSession(summary, modelId);
    },
    [projectId],
  );

  const refreshSession = useCallback(
    async (sid: string) => {
      const remote = await chatApi.getChatSession(projectId, sid);
      const state: SessionState = sessionFromApi(
        remote,
        capabilities.default_model_id,
      );
      applySession(state);
      if (sid === activeSessionId) {
        setItems(sessionItems(state));
      }
      return state;
    },
    [activeSessionId, applySession, capabilities.default_model_id, projectId],
  );

  const persistSessionMeta = useCallback(
    async (sid: string, patch: Partial<SessionState>) => {
      const session = sessions.find((s) => s.id === sid);
      if (!session) return;
      const next = { ...session, ...patch };
      setSessions((prev) =>
        prev.map((s) => (s.id === sid ? { ...s, ...patch } : s)),
      );
      await chatApi.patchChatSession(projectId, sid, {
        title: next.title,
        model_id: next.modelId,
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
          const firstState: SessionState = sessionFromApi(
            first,
            caps.default_model_id,
          );
          const rest: SessionState[] = summaries
            .slice(1)
            .map((s) => summaryToSession(s, caps.default_model_id));
          setSessions([firstState, ...rest]);
          setActiveSessionId(firstState.id);
          setItems(sessionItems(firstState));
        } else {
          const created = await chatApi.createChatSession(projectId, {
            title: "新对话",
            model_id: caps.default_model_id,
          });
          if (cancelled) return;
          const empty = summaryToSession(created, caps.default_model_id);
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
        } catch {
          setItems([]);
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
      const created = await createServerSession(capabilities.default_model_id);
      const { sessions: summaries } = await chatApi.listChatSessions(projectId);
      setSessions((prev) => {
        const byId = new Map(prev.map((s) => [s.id, s]));
        return (summaries ?? []).map((summary) => {
          const existing = byId.get(summary.id);
          if (existing && existing.nodes.length > 0) {
            return {
              ...existing,
              title: summary.title || existing.title,
              modelId: summary.model_id || existing.modelId,
              activeLeafId: summary.active_leaf_id ?? existing.activeLeafId,
              updatedAt: summary.updated_at,
            };
          }
          return summaryToSession(summary, capabilities.default_model_id);
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
    capabilities.default_model_id,
    createServerSession,
    items.length,
    loading,
    projectId,
    sessions,
  ]);

  const handleSessionModelChange = useCallback(
    async (modelId: string) => {
      if (!activeSessionId) return;
      await persistSessionMeta(activeSessionId, { modelId });
    },
    [activeSessionId, persistSessionMeta],
  );

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
          } catch {
            setItems([]);
          }
        } else {
          setItems(sessionItems(target));
        }
        return;
      }
      try {
        const created = await createServerSession(
          capabilities.default_model_id,
        );
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
      capabilities.default_model_id,
      createServerSession,
      projectId,
      refreshSession,
      sessions,
    ],
  );

  const conversationMenu: ConversationsProps["menu"] = useCallback(
    (conversation: ConversationItemType) => ({
      items: [
        {
          label: "重命名",
          key: "rename",
          icon: <EditOutlined />,
        },
        {
          label: "删除",
          key: "delete",
          icon: <DeleteOutlined />,
          danger: true,
        },
      ],
      onClick: ((info) => {
        info.domEvent.stopPropagation();
        const sid = String(conversation.key);
        if (info.key === "rename") openRename(sid);
        if (info.key === "delete") {
          Modal.confirm({
            title: "删除对话",
            content: "确定删除该对话？此操作不可恢复。",
            okText: "删除",
            okType: "danger",
            cancelText: "取消",
            onOk: () => deleteSession(sid),
          });
        }
      }) as MenuProps["onClick"],
    }),
    [deleteSession, openRename],
  );

  const conversationItems = useMemo(
    () =>
      sessions.map((s) => ({
        key: s.id,
        label: s.title || "新对话",
        icon: <MessageOutlined />,
      })),
    [sessions],
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
      const modelId = session?.modelId || capabilities.default_model_id;
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
          modelId,
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
        if (!cancelledRef.current) {
          setLoading(false);
        }
      }
    },
    [
      activeSessionId,
      capabilities.default_model_id,
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

  if (booting) {
    return (
      <Flex
        align="center"
        justify="center"
        style={{ height: `calc(100vh - ${headerHeight}px)` }}
      >
        <Spin />
      </Flex>
    );
  }

  return (
    <Flex
      style={{
        height: `calc(100vh - ${headerHeight}px)`,
        minHeight: 0,
        overflow: "hidden",
        background: token.colorBgContainer,
      }}
    >
      <Conversations
        activeKey={activeSessionId}
        onActiveChange={(key) => switchSession(String(key))}
        items={conversationItems}
        menu={conversationMenu}
        creation={{
          onClick: createNewChat,
          disabled: loading,
        }}
        style={{
          width: 260,
          flexShrink: 0,
          height: "100%",
          borderRight: `1px solid ${token.colorBorderSecondary}`,
          background: token.colorBgContainer,
          padding: "12px 8px",
        }}
      />
      <Flex
        vertical
        style={{ flex: 1, minWidth: 0, minHeight: 0, overflow: "hidden" }}
      >
        {modelOptions.length > 0 ? (
          <Flex
            align="center"
            gap={8}
            style={{
              padding: "8px 24px 0",
              flexShrink: 0,
            }}
          >
            <Typography.Text type="secondary">会话模型</Typography.Text>
            <Select
              size="small"
              style={{ minWidth: 220 }}
              disabled={loading}
              value={sessionModelId}
              options={modelOptions.map((m) => ({
                value: m.id,
                label: m.name,
              }))}
              onChange={handleSessionModelChange}
            />
          </Flex>
        ) : null}
        {error && (
          <Alert
            type="error"
            message={error}
            style={{ margin: "8px 24px 0", flexShrink: 0 }}
          />
        )}
        <MatrixAiChat
          items={items}
          loading={loading}
          projectId={projectId}
          capabilities={{
            multimodal: sessionModelCaps.multimodal,
            attachment_types: sessionModelCaps.attachment_types,
          }}
          modelLabel={sessionModelCaps.model_name}
          multimodalHint={
            sessionModelCaps.multimodal
              ? `当前模型支持：${sessionModelCaps.attachment_types
                  .map((t) => (t === "image" ? "图片" : "txt/md"))
                  .join("、")}`
              : undefined
          }
          welcomeTitle="开始对话"
          welcomeDescription={
            sessionModelCaps.model_name
              ? `当前模型：${sessionModelCaps.model_name}`
              : "请在系统配置中启用 AI 模型"
          }
          prompts={DEFAULT_PROMPTS}
          onSubmit={send}
          onCancel={loading ? handleCancel : undefined}
          onResendUserMessage={resendUserMessage}
          onToggleMessageActivity={toggleMessageActivity}
          activitySlot={
            loading && activityState ? (
              <div style={{ margin: "0 24px 12px" }}>
                <RunActivityPanel state={activityState} running compact />
              </div>
            ) : null
          }
          style={{ flex: 1, minHeight: 0, height: "100%" }}
        />
      </Flex>
      <Modal
        title="重命名对话"
        open={renameOpen}
        okText="保存"
        cancelText="取消"
        onOk={confirmRename}
        onCancel={() => setRenameOpen(false)}
        okButtonProps={{ disabled: !renameValue.trim() }}
      >
        <Input
          value={renameValue}
          onChange={(e) => setRenameValue(e.target.value)}
          placeholder="对话标题"
          onPressEnter={confirmRename}
        />
      </Modal>
    </Flex>
  );
}
