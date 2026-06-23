import { useCallback, useMemo, useRef, useState, type ReactNode } from "react";
import { Avatar, Button, Flex, Image, Tag, theme } from "antd";
import type { GetRef } from "antd";
import {
  PaperClipOutlined,
  RobotOutlined,
  UserOutlined,
} from "@ant-design/icons";
import {
  Attachments,
  Bubble,
  Sender,
} from "@ant-design/x";
import type { AttachmentsProps } from "@ant-design/x";
import type { BubbleItemType } from "@ant-design/x";
import MarkdownView from "@/components/docs/MarkdownView";

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
}

export interface ChatCapabilities {
  multimodal: boolean;
  attachment_types: string[];
}

export interface MatrixAiChatProps {
  items?: AiMessage[];
  loading?: boolean;
  placeholder?: string;
  capabilities?: ChatCapabilities;
  onSubmit: (
    message: string,
    attachments?: ChatAttachment[],
  ) => void | Promise<void>;
  onCancel?: () => void;
  className?: string;
  style?: React.CSSProperties;
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

function detectAttachmentType(
  file: File,
  allowed: string[],
): string | null {
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

function renderMessageContent(item: AiMessage) {
  const hasAttachments = (item.attachments?.length ?? 0) > 0;
  if (!hasAttachments) return item.content;
  return (
    <Flex vertical gap={8}>
      {item.content ? <div>{item.content}</div> : null}
      {item.attachments?.map((att) => {
        if (att.type === "image") {
          return (
            <Image
              key={`${att.name}-${att.mime_type}`}
              src={`data:${att.mime_type};base64,${att.data}`}
              alt={att.name}
              style={{ maxWidth: 240, maxHeight: 240, borderRadius: 4 }}
            />
          );
        }
        return (
          <Tag key={`${att.name}-${att.type}`}>📄 {att.name}</Tag>
        );
      })}
    </Flex>
  );
}

export default function MatrixAiChat({
  items = [],
  loading = false,
  placeholder = "输入消息…",
  capabilities,
  onSubmit,
  onCancel,
  className,
  style,
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

  const beforeUpload = useCallback(
    (file: File) => {
      const type = detectAttachmentType(file, allowedTypes);
      if (!type) {
        return false;
      }
      const maxBytes = type === "image" ? IMAGE_MAX_BYTES : DOCUMENT_MAX_BYTES;
      if (file.size > maxBytes) {
        return false;
      }
      attachmentsRef.current?.upload(file);
      return false;
    },
    [allowedTypes],
  );

  const bubbleItems: BubbleItemType[] = items.map((item) => ({
    key: item.key,
    role: item.role,
    content:
      item.role === "user" ? renderMessageContent(item) : item.content,
    loading: item.loading,
    streaming: item.role === "ai" && !!item.loading,
  }));

  const bubbleRole = useMemo(
    () => ({
      user: {
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
            background: token.colorPrimary,
            color: token.colorTextLightSolid,
          },
        },
      },
      ai: (data: BubbleItemType) => ({
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
            background: token.colorFillTertiary,
            maxWidth: "100%",
          },
        },
        contentRender: (content: unknown) => {
          if (typeof content !== "string") return content as ReactNode;
          if (!content && data.loading) return content;
          return (
            <MarkdownView content={content} streaming={!!data.loading} />
          );
        },
      }),
    }),
    [token],
  );

  async function handleSubmit(message: string) {
    const text = message.trim();
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
    const type = detectAttachmentType(firstFile, allowedTypes);
    if (!type) return;
    const maxBytes = type === "image" ? IMAGE_MAX_BYTES : DOCUMENT_MAX_BYTES;
    if (firstFile.size > maxBytes) return;
    attachmentsRef.current?.upload(firstFile);
  }

  return (
    <div
      className={className}
      style={{
        display: "grid",
        gridTemplateRows: "1fr auto",
        height: "100%",
        minHeight: 0,
        background: token.colorBgContainer,
        ...style,
      }}
    >
      <div style={{ minHeight: 0, overflow: "auto", padding: "16px 24px" }}>
        {items.length > 0 && (
          <Bubble.List
            items={bubbleItems}
            autoScroll
            role={bubbleRole}
          />
        )}
      </div>
      <div
        style={{
          padding: "12px 24px 16px",
          borderTop: `1px solid ${token.colorBorderSecondary}`,
          background: token.colorBgContainer,
        }}
      >
        <Sender
          style={{ width: "100%" }}
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
                    description: "点击或拖拽文件到此处",
                  }}
                />
              </Sender.Header>
            ) : undefined
          }
          styles={{ input: { minHeight: 40 } }}
        />
      </div>
    </div>
  );
}
