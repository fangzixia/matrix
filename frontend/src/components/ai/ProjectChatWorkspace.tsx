import { useCallback, useMemo, type CSSProperties, type ReactNode } from "react";
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
  type ChatPromptItem,
} from "@/components/ai/MatrixAiChat";
import RunActivityPanel from "@/components/ai/RunActivityPanel";
import { useProjectChat } from "@/hooks/useProjectChat";

export interface ProjectChatWorkspaceProps {
  projectId: string;
  prompts?: ChatPromptItem[];
  welcomeTitle?: string;
  welcomeDescription?: string;
  footer?: ReactNode;
  style?: CSSProperties;
  className?: string;
}

export default function ProjectChatWorkspace({
  projectId,
  prompts,
  welcomeTitle = "开始对话",
  welcomeDescription,
  footer,
  style,
  className,
}: ProjectChatWorkspaceProps) {
  const { token } = theme.useToken();
  const headerHeight = token.Layout?.headerHeight ?? 48;

  const {
    booting,
    error,
    loading,
    items,
    sessions,
    activeSessionId,
    sessionModelId,
    sessionModelCaps,
    modelOptions,
    activityState,
    switchSession,
    createNewChat,
    handleSessionModelChange,
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
  } = useProjectChat(projectId);

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

  const resolvedWelcomeDescription =
    welcomeDescription ??
    (sessionModelCaps.model_name
      ? `当前模型：${sessionModelCaps.model_name}`
      : "请在系统配置中启用 AI 模型");

  if (booting) {
    return (
      <Flex
        align="center"
        justify="center"
        className={className}
        style={{
          height: `calc(100vh - ${headerHeight}px)`,
          ...style,
        }}
      >
        <Spin />
      </Flex>
    );
  }

  return (
    <Flex
      className={className}
      style={{
        height: "100%",
        minHeight: 0,
        overflow: "hidden",
        background: token.colorBgContainer,
        ...style,
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
          welcomeTitle={welcomeTitle}
          welcomeDescription={resolvedWelcomeDescription}
          prompts={prompts}
          onSubmit={send}
          onCancel={loading ? handleCancel : undefined}
          onResendUserMessage={resendUserMessage}
          onToggleMessageActivity={toggleMessageActivity}
          activitySlot={
            activityState ? (
              <div aria-live="polite" style={{ margin: "0 24px 12px" }}>
                <RunActivityPanel
                  state={activityState}
                  running={loading}
                  compact
                  projectId={projectId}
                />
              </div>
            ) : null
          }
          style={{ flex: 1, minHeight: 0, height: "100%" }}
        />
        {footer ? (
          <div
            style={{
              flexShrink: 0,
              padding: "12px 24px 16px",
              borderTop: `1px solid ${token.colorBorderSecondary}`,
              background: token.colorBgContainer,
            }}
          >
            {footer}
          </div>
        ) : null}
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
