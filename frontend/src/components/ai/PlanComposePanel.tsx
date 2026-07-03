import type { CSSProperties, ReactNode } from "react";
import { Alert, Flex, theme } from "antd";
import MatrixAiChat, {
  type ChatPromptItem,
} from "@/components/ai/MatrixAiChat";
import RunActivityPanel from "@/components/ai/RunActivityPanel";
import { usePlanCompose } from "@/hooks/usePlanCompose";

export interface PlanComposePanelProps {
  projectId: string;
  filePath: string;
  prompts?: ChatPromptItem[];
  welcomeTitle?: string;
  welcomeDescription?: string;
  footer?: ReactNode;
  onRunComplete?: () => void;
  style?: CSSProperties;
  className?: string;
}

export default function PlanComposePanel({
  projectId,
  filePath,
  prompts,
  welcomeTitle = "编写计划",
  welcomeDescription,
  footer,
  onRunComplete,
  style,
  className,
}: PlanComposePanelProps) {
  const { token } = theme.useToken();

  const {
    error,
    loading,
    items,
    activityState,
    send,
    resendUserMessage,
    toggleMessageActivity,
    handleCancel,
  } = usePlanCompose(projectId, filePath, onRunComplete);

  return (
    <Flex
      vertical
      className={className}
      style={{
        height: "100%",
        minHeight: 0,
        overflow: "hidden",
        background: token.colorBgLayout,
        ...style,
      }}
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
        projectId={projectId}
        welcomeTitle={welcomeTitle}
        welcomeDescription={welcomeDescription}
        prompts={prompts}
        placeholder="描述计划需求…"
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
  );
}
