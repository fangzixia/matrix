import { useCallback, useMemo, useRef, useState, type ReactNode } from "react";
import {
  Avatar,
  Button,
  Flex,
  Image,
  Spin,
  Splitter,
  Tag,
  Typography,
  message,
  theme,
} from "antd";
import type { GetRef } from "antd";
import type { GlobalToken } from "antd/es/theme/interface";
import {
  PaperClipOutlined,
  RobotOutlined,
  SyncOutlined,
  ToolOutlined,
  UserOutlined,
} from "@ant-design/icons";
import {
  Actions,
  Attachments,
  Bubble,
  Prompts,
  Sender,
  Welcome,
} from "@ant-design/x";
import type { AttachmentsProps } from "@ant-design/x";
import type { BubbleItemType } from "@ant-design/x";
import MarkdownView from "@/components/docs/MarkdownView";
import RunActivityPanel from "@/components/ai/RunActivityPanel";
import {
  isRunViewTerminal,
  useRunActivityView,
} from "@/hooks/useRunActivityView";

type AttachmentsRef = GetRef<typeof Attachments>;

export type AiMessageRole = "user" | "ai" | "system";

export interface ChatAttachment {
  type: string;
  mime_type: string;
  name: string;
  data: string;
}

export interface AiMessage {
  key: string | number;
  role: AiMessageRole;
  content: string;
  loading?: boolean;
  attachments?: ChatAttachment[];
  runId?: string;
  parentId?: string | null;
  activityExpanded?: boolean;
}

export interface ChatCapabilities {
  multimodal: boolean;
  attachment_types: string[];
}

export interface ChatPromptItem {
  key: string;
  label: string;
  description?: string;
}

export interface MatrixAiChatProps {
  items?: AiMessage[];
  loading?: boolean;
  placeholder?: string;
  capabilities?: ChatCapabilities;
  modelLabel?: string;
  multimodalHint?: string;
  welcomeTitle?: string;
  welcomeDescription?: string;
  prompts?: ChatPromptItem[];
  projectId?: string;
  onSubmit: (
    message: string,
    attachments?: ChatAttachment[],
  ) => void | Promise<void>;
  onCancel?: () => void;
  onResendUserMessage?: (messageKey: string | number) => void;
  onToggleMessageActivity?: (messageKey: string | number) => void;
  className?: string;
  style?: React.CSSProperties;
  activitySlot?: ReactNode;
}

type AttachmentItem = NonNullable<AttachmentsProps["items"]>[number];

const IMAGE_MAX_BYTES = 10 * 1024 * 1024;
const DOCUMENT_MAX_BYTES = 5 * 1024 * 1024;

function acceptFromTypes(types: string[]): string {
  const parts: string[] = [];
  if (types.includes("image")) {
    parts.push("image/png", "image/jpeg", "image/gif", "image/webp");
  }
  if (types.includes("document")) {
    parts.push(".txt", ".md", "text/plain", "text/markdown");
  }
  return parts.join(",");
}

function detectAttachmentType(file: File, allowed: string[]): string | null {
  if (allowed.includes("image") && file.type.startsWith("image/")) {
    return "image";
  }
  if (allowed.includes("document")) {
    const ext = file.name.split(".").pop()?.toLowerCase();
    if (ext === "txt" || ext === "md") return "document";
    if (file.type === "text/plain" || file.type === "text/markdown") {
      return "document";
    }
  }
  return null;
}

function readFileAsBase64(file: File): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.onload = () => {
      const result = reader.result as string;
      const base64 = result.includes(",") ? result.split(",")[1] : result;
      resolve(base64);
    };
    reader.onerror = () => reject(reader.error);
    reader.readAsDataURL(file);
  });
}

async function fileToChatAttachment(
  file: File,
  type: string,
): Promise<ChatAttachment> {
  if (type === "image") {
    return {
      type,
      mime_type: file.type || "image/png",
      name: file.name,
      data: await readFileAsBase64(file),
    };
  }
  return {
    type,
    mime_type: file.type || "text/plain",
    name: file.name,
    data: await file.text(),
  };
}

function renderUserAttachments(item: AiMessage, token: GlobalToken) {
  if (!item.attachments?.length) return null;
  return (
    <Flex vertical gap={8}>
      {item.attachments.map((att) => {
        if (att.type === "image") {
          return (
            <Image
              key={`${att.name}-${att.mime_type}`}
              src={`data:${att.mime_type};base64,${att.data}`}
              alt={att.name}
              style={{ maxWidth: 240, maxHeight: 240, borderRadius: token.borderRadius }}
            />
          );
        }
        return (
          <Tag key={`${att.name}-${att.type}`} icon={<PaperClipOutlined />}>
            {att.name}
          </Tag>
        );
      })}
    </Flex>
  );
}

function MessageActivityBlock({
  projectId,
  runId,
}: {
  projectId: string;
  runId: string;
}) {
  const { state, loading, error, disconnected } = useRunActivityView(
    projectId,
    runId,
    { live: true, mode: "chat" },
  );

  if (loading) return <Spin size="small" />;
  if (error) {
    return (
      <Typography.Text type="danger" style={{ fontSize: 12 }}>
        {error}
      </Typography.Text>
    );
  }
  if (!state) return null;
  return (
    <Flex vertical gap={8}>
      {disconnected && !isRunViewTerminal(state) ? (
        <Typography.Text type="secondary" style={{ fontSize: 12 }}>
          连接中断，正在等待浏览器自动重连…
        </Typography.Text>
      ) : null}
      <RunActivityPanel
        state={state}
        running={!isRunViewTerminal(state)}
        projectId={projectId}
      />
    </Flex>
  );
}

export default function MatrixAiChat({
  items = [],
  loading = false,
  placeholder = "输入消息…",
  capabilities,
  modelLabel,
  multimodalHint,
  welcomeTitle = "开始对话",
  welcomeDescription,
  prompts,
  projectId,
  onSubmit,
  onCancel,
  onResendUserMessage,
  onToggleMessageActivity,
  className,
  style,
  activitySlot,
}: MatrixAiChatProps) {
  const { token } = theme.useToken();
  const [inputValue, setInputValue] = useState("");
  const [headerOpen, setHeaderOpen] = useState(false);
  const [attachmentItems, setAttachmentItems] = useState<AttachmentItem[]>([]);
  const attachmentsRef = useRef<AttachmentsRef>(null);

  const showAttachments = capabilities?.multimodal === true;
  const allowedTypes = capabilities?.attachment_types ?? [];

  const accept = useMemo(
    () => acceptFromTypes(allowedTypes),
    [allowedTypes],
  );

  const itemsByKey = useMemo(
    () => new Map(items.map((item) => [item.key, item])),
    [items],
  );

  const validateFile = useCallback(
    (file: File): string | null => {
      const type = detectAttachmentType(file, allowedTypes);
      if (!type) {
        return "不支持的文件类型，仅支持图片或 txt/md 文档";
      }
      const maxBytes = type === "image" ? IMAGE_MAX_BYTES : DOCUMENT_MAX_BYTES;
      if (file.size > maxBytes) {
        const mb = Math.round(maxBytes / (1024 * 1024));
        return `文件过大，${type === "image" ? "图片" : "文档"}最大 ${mb} MB`;
      }
      return null;
    },
    [allowedTypes],
  );

  const beforeUpload = useCallback(
    (file: File) => {
      const err = validateFile(file);
      if (err) {
        message.warning(err);
        return false;
      }
      attachmentsRef.current?.upload(file);
      return false;
    },
    [validateFile],
  );

  const bubbleItems: BubbleItemType[] = items.map((item) => ({
    key: item.key,
    role: item.role,
    content: item.content,
    loading: item.loading,
    streaming: item.role === "ai" && !!item.loading,
  }));

  const bubbleRole = useMemo(
    () => ({
      user: (data: BubbleItemType) => {
        const src = itemsByKey.get(data.key ?? "");
        const hasAttachments = (src?.attachments?.length ?? 0) > 0;
        const showResend =
          !!onResendUserMessage &&
          !loading &&
          data.key != null &&
          (src?.content?.trim() || hasAttachments);
        const footerParts: ReactNode[] = [];
        if (hasAttachments && src) footerParts.push(renderUserAttachments(src, token));
        if (showResend) {
          footerParts.push(
            <Actions
              key="resend"
              items={[
                {
                  key: "resend",
                  label: "重新发送",
                  icon: <SyncOutlined />,
                  onItemClick: () => onResendUserMessage(data.key!),
                },
              ]}
            />,
          );
        }
        return {
          placement: "end" as const,
          variant: "filled" as const,
          avatar: (
            <Avatar
              size={32}
              icon={<UserOutlined />}
              style={{ backgroundColor: token.colorPrimary }}
            />
          ),
          styles: {
            content: {
              background: token.colorPrimaryBg,
              color: token.colorText,
              border: `1px solid ${token.colorPrimaryBorder}`,
              borderRadius: token.borderRadiusLG,
            },
          },
          footer: footerParts.length ? (
            <Flex vertical gap={4} align="end">
              {footerParts}
            </Flex>
          ) : undefined,
          footerPlacement: "outer-end" as const,
        };
      },
      ai: (data: BubbleItemType) => {
        const src = itemsByKey.get(data.key ?? "");
        const textContent =
          typeof data.content === "string" ? data.content : "";
        const showActions = !data.loading && textContent;
        return {
          placement: "start" as const,
          variant: "filled" as const,
          avatar: (
            <Avatar
              size={32}
              icon={<RobotOutlined />}
              style={{
                backgroundColor: token.colorFillSecondary,
                color: token.colorText,
              }}
            />
          ),
          styles: {
            content: {
              background: token.colorBgContainer,
              border: `1px solid ${token.colorBorderSecondary}`,
              borderRadius: token.borderRadiusLG,
              maxWidth: "100%",
            },
          },
          contentRender: (content: unknown) => {
            if (typeof content !== "string") return content as ReactNode;
            if (!content && data.loading) return content;
            return (
              <Flex vertical gap={8} style={{ width: "100%" }}>
                <MarkdownView
                  content={content}
                  variant="chat"
                  streaming={!!data.loading}
                />
                {src?.activityExpanded && src.runId && projectId ? (
                  <MessageActivityBlock
                    projectId={projectId}
                    runId={src.runId}
                  />
                ) : null}
              </Flex>
            );
          },
          footer: showActions ? (
            <Actions
              items={[
                {
                  key: "copy",
                  actionRender: <Actions.Copy text={textContent} />,
                },
                ...(src?.runId && onToggleMessageActivity && data.key != null
                  ? [
                      {
                        key: "tools",
                        label: src.activityExpanded ? "收起过程" : "执行过程",
                        icon: <ToolOutlined />,
                        onItemClick: () =>
                          onToggleMessageActivity(data.key!),
                      },
                    ]
                  : []),
              ]}
            />
          ) : undefined,
          footerPlacement: "outer-start" as const,
        };
      },
    }),
    [
      itemsByKey,
      loading,
      onResendUserMessage,
      onToggleMessageActivity,
      projectId,
      token,
    ],
  );

  async function handleSubmit(msg: string) {
    const text = msg.trim();
    const files = attachmentItems.filter((f) => f.originFileObj);
    const attachments: ChatAttachment[] = [];
    for (const item of files) {
      const file = item.originFileObj as File;
      const type = detectAttachmentType(file, allowedTypes);
      if (!type) continue;
      attachments.push(await fileToChatAttachment(file, type));
    }
    if (!text && attachments.length === 0) return;

    setInputValue("");
    setAttachmentItems([]);
    setHeaderOpen(false);
    await onSubmit(text, attachments.length ? attachments : undefined);
  }

  function handlePasteFile(files: FileList) {
    if (!showAttachments || files.length === 0) return;
    const firstFile = files[0];
    const err = validateFile(firstFile);
    if (err) {
      message.warning(err);
      return;
    }
    attachmentsRef.current?.upload(firstFile);
  }

  const attachmentHint = useMemo(() => {
    if (!showAttachments) return undefined;
    const parts: string[] = [];
    if (allowedTypes.includes("image")) parts.push("图片");
    if (allowedTypes.includes("document")) parts.push("txt/md");
    return parts.length ? `支持附件：${parts.join("、")}` : undefined;
  }, [showAttachments, allowedTypes]);

  const messageList = (
    <>
      {items.length === 0 ? (
        <Flex vertical gap={16} align="center" style={{ paddingTop: 48 }}>
          <Welcome
            icon={<RobotOutlined />}
            title={welcomeTitle}
            description={
              welcomeDescription ||
              (modelLabel ? `当前模型：${modelLabel}` : undefined)
            }
            variant="borderless"
          />
          {multimodalHint || attachmentHint ? (
            <Typography.Text type="secondary">
              {multimodalHint || attachmentHint}
            </Typography.Text>
          ) : null}
          {prompts && prompts.length > 0 ? (
            <Prompts
              title="你可以问我"
              items={prompts.map((p) => ({
                key: p.key,
                label: p.label,
                description: p.description,
              }))}
              onItemClick={(info) => {
                const label =
                  typeof info.data.label === "string"
                    ? info.data.label
                    : String(info.data.label ?? "");
                setInputValue(label);
              }}
              styles={{
                list: { gap: 12, maxWidth: 720, width: "100%" },
                item: {
                  flex: "1 1 200px",
                  border: `1px solid ${token.colorBorderSecondary}`,
                  borderRadius: token.borderRadiusLG,
                  background: token.colorBgContainer,
                  boxShadow: token.boxShadowTertiary,
                  padding: "14px 16px",
                },
              }}
              wrap
            />
          ) : null}
        </Flex>
      ) : (
        <Bubble.List items={bubbleItems} autoScroll role={bubbleRole} />
      )}
      {activitySlot}
    </>
  );

  const senderPanel = (
    <div
      style={{
        height: "100%",
        boxSizing: "border-box",
        padding: "12px 24px 16px",
        background: token.colorBgContainer,
        borderTop: `1px solid ${token.colorBorderSecondary}`,
        overflow: "auto",
      }}
    >
      <Sender
        style={{
          width: "100%",
          border: `1px solid ${token.colorBorderSecondary}`,
          borderRadius: token.borderRadiusLG,
          boxShadow: token.boxShadowTertiary,
          background: token.colorBgContainer,
        }}
        value={inputValue}
        onChange={setInputValue}
        loading={loading}
        disabled={loading}
        placeholder={loading ? "AI 正在回复…" : placeholder}
        onSubmit={handleSubmit}
        onCancel={onCancel}
        onPasteFile={
          showAttachments && !loading ? handlePasteFile : undefined
        }
        prefix={
          showAttachments ? (
            <Button
              type="text"
              disabled={loading}
              icon={<PaperClipOutlined />}
              onClick={() => setHeaderOpen((open) => !open)}
              aria-label="添加附件"
            />
          ) : undefined
        }
        header={
          showAttachments && !loading ? (
            <Sender.Header
              title="附件"
              open={headerOpen}
              onOpenChange={setHeaderOpen}
            >
              <Attachments
                ref={attachmentsRef}
                items={attachmentItems}
                onChange={({ fileList }) => setAttachmentItems(fileList)}
                beforeUpload={beforeUpload}
                accept={accept}
                maxCount={5}
                placeholder={{
                  icon: <PaperClipOutlined />,
                  title: "上传附件",
                  description: attachmentHint || "点击或拖拽文件到此处",
                }}
              />
            </Sender.Header>
          ) : undefined
        }
        styles={{ input: { minHeight: 40 } }}
      />
    </div>
  );

  return (
    <Splitter
      vertical
      className={className}
      style={{
        height: "100%",
        minHeight: 0,
        background: token.colorBgLayout,
        ...style,
      }}
    >
      <Splitter.Panel defaultSize="70%" min="120px">
        <div
          style={{
            height: "100%",
            minHeight: 0,
            overflow: "auto",
            padding: "16px 24px",
            boxSizing: "border-box",
            background: token.colorBgLayout,
          }}
        >
          {messageList}
        </div>
      </Splitter.Panel>
      <Splitter.Panel min="96px">{senderPanel}</Splitter.Panel>
    </Splitter>
  );
}
