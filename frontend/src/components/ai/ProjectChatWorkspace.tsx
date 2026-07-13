import { useCallback, useMemo, type CSSProperties, type ReactNode } from "react";
import {
  Alert,
  Flex,
  Input,
  Modal,
  Spin,
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
import RunActivityPanel, {
  runViewHasActivityPanel,
} from "@/components/ai/RunActivityPanel";
import { useProjectChat } from "@/hooks/useProjectChat";
import { MATRIX_LAYOUT } from "@/theme/layout";

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
    (capabilities.model_name
      ? `当前模型：${capabilities.model_name}`
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
        background: token.colorBgLayout,
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
        styles={{
          root: {
            background: token.colorFillAlter,
          },
          creation: {
            borderRadius: token.borderRadius,
            fontWeight: 500,
          },
          item: {
            borderRadius: token.borderRadius,
          },
        }}
        style={{
          width: MATRIX_LAYOUT.chatSiderWidth,
          flexShrink: 0,
          height: "100%",
          borderRight: `1px solid ${token.colorBorder}`,
          padding: `${token.paddingSM}px ${token.paddingXS}px`,
        }}
      />
      <Flex
        vertical
        style={{
          flex: 1,
          minWidth: 0,
          minHeight: 0,
          overflow: "hidden",
          background: token.colorBgContainer,
        }}
      >
        {error && (
          <Alert
            type="error"
            title={error}
            style={{
              margin: `${token.marginXS}px ${token.paddingLG}px 0`,
              flexShrink: 0,
            }}
          />
        )}
        <MatrixAiChat
          items={items}
          loading={loading}
          projectId={projectId}
          capabilities={{
            multimodal: capabilities.multimodal,
            attachment_types: capabilities.attachment_types,
          }}
          modelLabel={capabilities.model_name}
          multimodalHint={
            capabilities.multimodal
              ? `当前模型支持：${capabilities.attachment_types
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
            activityState && runViewHasActivityPanel(activityState, true) ? (
              <div aria-live="polite">
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
          <Flex
            style={{
              flexShrink: 0,
              padding: `${token.paddingSM}px ${token.paddingLG}px ${token.padding}px`,
              borderTop: `1px solid ${token.colorBorderSecondary}`,
              background: token.colorBgContainer,
            }}
          >
            {footer}
          </Flex>
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
