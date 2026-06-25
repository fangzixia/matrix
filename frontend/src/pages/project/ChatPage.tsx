import { useCallback, useEffect, useMemo, useState } from "react";
import { useParams } from "react-router-dom";
import { Alert, Flex, Spin, theme } from "antd";
import { MessageOutlined } from "@ant-design/icons";
import { Conversations } from "@ant-design/x";
import MatrixAiChat, {
  type AiMessage,
  type ChatAttachment,
  type ChatCapabilities,
} from "@/components/ai/MatrixAiChat";
import RunActivityPanel from "@/components/ai/RunActivityPanel";
import * as chatApi from "@/api/chat";
import { useTaskStream } from "@/hooks/useTaskStream";
import type { RunActivityState } from "@/types/runStream";

interface SessionState {
  id: string;
  title: string;
  messages: chatApi.ChatMessage[];
  updatedAt?: string;
  isLocal?: boolean;
}

function messagesToItems(messages: chatApi.ChatMessage[]): AiMessage[] {
  return messages.map((m, i) => ({
    key: `hist-${i}`,
    role: m.role === "user" ? "user" : "ai",
    content: m.content,
    attachments: m.attachments,
  }));
}

function itemsToMessages(items: AiMessage[]): chatApi.ChatMessage[] {
  return items
    .filter((item) => (item.content || item.attachments?.length) && !item.loading)
    .map((item) => ({
      role: item.role === "user" ? ("user" as const) : ("assistant" as const),
      content: item.content,
      attachments: item.attachments,
    }));
}

function sessionTitle(messages: chatApi.ChatMessage[]): string {
  const first = messages.find((m) => m.role === "user" && m.content.trim());
  if (first?.content) return first.content.trim().slice(0, 80);
  return "新对话";
}

function hasMessages(messages: chatApi.ChatMessage[]): boolean {
  return messages.some((m) => m.content.trim() || (m.attachments?.length ?? 0) > 0);
}

function fromApiSession(session: chatApi.ChatSession): SessionState {
  return {
    id: session.id,
    title: session.title || "对话",
    messages: chatApi.parseChatMessages(session.messages),
    updatedAt: session.updated_at,
  };
}

export default function ChatPage() {
  const { id: projectId = "" } = useParams();
  const { token } = theme.useToken();
  const headerHeight = token.Layout?.headerHeight ?? 48;
  const { streamTask } = useTaskStream();
  const [sessions, setSessions] = useState<SessionState[]>([]);
  const [activeSessionId, setActiveSessionId] = useState("");
  const [items, setItems] = useState<AiMessage[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const [booting, setBooting] = useState(true);
  const [capabilities, setCapabilities] = useState<ChatCapabilities>({
    multimodal: false,
    attachment_types: [],
  });
  const [activityState, setActivityState] = useState<RunActivityState | null>(
    null,
  );

  const mergeSessionMessages = useCallback(
    (sid: string, nextItems: AiMessage[]) => {
      const messages = itemsToMessages(nextItems);
      setSessions((prev) => {
        const exists = prev.some((s) => s.id === sid);
        const title = sessionTitle(messages);
        const updatedAt = new Date().toISOString();
        if (!exists) {
          return [
            {
              id: sid,
              title,
              messages,
              updatedAt,
              isLocal: false,
            },
            ...prev,
          ];
        }
        return prev.map((s) =>
          s.id === sid
            ? { ...s, title, messages, updatedAt, isLocal: false }
            : s,
        );
      });
      return messages;
    },
    [],
  );

  const persistSession = useCallback(
    async (nextItems: AiMessage[], sid: string) => {
      const messages = mergeSessionMessages(sid, nextItems);
      await chatApi.saveChatSessions(projectId, [
        {
          id: sid,
          title: sessionTitle(messages),
          messages,
        },
      ]);
    },
    [mergeSessionMessages, projectId],
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
        setCapabilities({
          multimodal: caps.multimodal,
          attachment_types: caps.attachment_types ?? [],
        });
        const loaded = (sessionsRes.sessions ?? []).map(fromApiSession);
        if (loaded.length) {
          setSessions(loaded);
          setActiveSessionId(loaded[0].id);
          setItems(messagesToItems(loaded[0].messages));
        } else {
          const newId = crypto.randomUUID();
          const empty: SessionState = {
            id: newId,
            title: "新对话",
            messages: [],
            isLocal: true,
          };
          setSessions([empty]);
          setActiveSessionId(newId);
          setItems([]);
        }
      } catch {
        if (!cancelled) {
          const newId = crypto.randomUUID();
          setSessions([
            { id: newId, title: "新对话", messages: [], isLocal: true },
          ]);
          setActiveSessionId(newId);
          setItems([]);
        }
      } finally {
        if (!cancelled) setBooting(false);
      }
    }
    load();
    return () => {
      cancelled = true;
    };
  }, [projectId]);

  const syncCurrentSession = useCallback(
    (nextItems: AiMessage[]) => {
      if (!activeSessionId) return;
      const messages = itemsToMessages(nextItems);
      setSessions((prev) =>
        prev.map((s) =>
          s.id === activeSessionId
            ? {
                ...s,
                messages,
                title: hasMessages(messages) ? sessionTitle(messages) : s.title,
              }
            : s,
        ),
      );
    },
    [activeSessionId],
  );

  const switchSession = useCallback(
    (sid: string) => {
      if (loading || sid === activeSessionId) return;
      syncCurrentSession(items);
      const target = sessions.find((s) => s.id === sid);
      if (!target) return;
      setActiveSessionId(sid);
      setItems(messagesToItems(target.messages));
      setError("");
    },
    [activeSessionId, items, loading, sessions, syncCurrentSession],
  );

  const createNewChat = useCallback(() => {
    if (loading) return;
    syncCurrentSession(items);
    const newId = crypto.randomUUID();
    setSessions((prev) => {
      const kept = prev.filter(
        (s) => !(s.isLocal && !hasMessages(s.messages)),
      );
      return [
        { id: newId, title: "新对话", messages: [], isLocal: true },
        ...kept,
      ];
    });
    setActiveSessionId(newId);
    setItems([]);
    setError("");
  }, [items, loading, syncCurrentSession]);

  const conversationItems = useMemo(
    () =>
      sessions.map((s) => ({
        key: s.id,
        label: s.title || "新对话",
        icon: <MessageOutlined />,
      })),
    [sessions],
  );

  async function send(message: string, attachments?: ChatAttachment[]) {
    const text = message.trim();
    if ((!text && !attachments?.length) || !activeSessionId || loading) return;
    setError("");
    setLoading(true);
    setActivityState(null);
    const sid = activeSessionId;
    const userKey = `user-${Date.now()}`;
    const aiKey = `ai-${Date.now()}`;
    const withUser: AiMessage[] = [
      ...items,
      {
        key: userKey,
        role: "user",
        content: text,
        attachments,
      },
      { key: aiKey, role: "ai", content: "", loading: true },
    ];
    setItems(withUser);
    try {
      const run = await chatApi.sendChatMessage(
        projectId,
        sid,
        text,
        attachments,
      );
      const reply = await streamTask(
        projectId,
        run.id,
        (_delta, full) => {
          setItems((prev) => {
            const next = prev.map((item) =>
              item.key === aiKey
                ? { ...item, content: full, loading: true }
                : item,
            );
            syncCurrentSession(next);
            return next;
          });
        },
        (state) => setActivityState(state),
      );
      setActivityState(null);
      const finalItems: AiMessage[] = withUser.map((item) =>
        item.key === aiKey
          ? { ...item, content: reply || "（无回复）", loading: false }
          : item,
      );
      setItems(finalItems);
      await persistSession(finalItems, sid);
    } catch (e) {
      setItems((prev) => prev.filter((item) => item.key !== aiKey));
      setError(e instanceof Error ? e.message : "发送失败");
    } finally {
      setLoading(false);
    }
  }

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
          capabilities={capabilities}
          onSubmit={send}
          activitySlot={
            loading && activityState ? (
              <div style={{ margin: "0 24px 12px" }}>
                <RunActivityPanel
                  state={activityState}
                  running
                  compact
                />
              </div>
            ) : null
          }
          style={{ flex: 1, minHeight: 0, height: "100%" }}
        />
      </Flex>
    </Flex>
  );
}
