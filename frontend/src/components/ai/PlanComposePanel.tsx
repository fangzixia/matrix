import type { CSSProperties, ReactNode } from "react";
import { Alert, Flex, theme } from "antd";
import MatrixAiChat, {
  type ChatPromptItem,
} from "@/components/ai/MatrixAiChat";
import RunActivityPanel, {
  runViewHasActivityPanel,
} from "@/components/ai/RunActivityPanel";
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
        welcomeTitle={welcomeTitle}
        welcomeDescription={welcomeDescription}
        prompts={prompts}
        placeholder="描述计划需求…"
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
  );
}
