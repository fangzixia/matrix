import { useMemo } from "react";
import type { ReactNode } from "react";
import { ApiOutlined, RobotOutlined, ToolOutlined } from "@ant-design/icons";
import { Collapse, Empty, Space, Tag, Typography } from "antd";
import {
  Bubble,
  CodeHighlighter,
  Think,
  ThoughtChain,
  Welcome,
} from "@ant-design/x";
import type { ThoughtChainItemType } from "@ant-design/x";
import type {
  RunViewState,
  SubagentView,
  ToolView,
  TurnView,
} from "@/types/runView";
import { formatAgentProgress, formatTurnLabel } from "@/utils/agentProgress";

export interface RunActivityPanelProps {
  state: RunViewState;
  running?: boolean;
  /** 紧凑模式：仅展示最后一轮与当前工具链。 */
  compact?: boolean;
}

function formatToolMessage(message?: string): {
  lang: string;
  content: string;
} {
  if (!message) return { lang: "text", content: "" };
  try {
    const parsed = JSON.parse(message);
    return { lang: "json", content: JSON.stringify(parsed, null, 2) };
  } catch {
    return { lang: "text", content: message };
  }
}

function toolTitle(tool: ToolView): string {
  if (tool.serverName) return `${tool.serverName} / ${tool.toolCallName}`;
  return tool.toolCallName;
}

function renderTurnBody(
  turn: TurnView,
  isLatest: boolean,
  running?: boolean,
) {
  const streaming = running && isLatest;
  const tools = turn.tools ?? [];
  return (
    <Space direction="vertical" size="middle" style={{ width: "100%" }}>
      {turn.thinking ? (
        <Think
          title="思考过程"
          defaultExpanded={streaming && turn.thinkingStreaming}
          loading={streaming && turn.thinkingStreaming}
          blink={streaming && turn.thinkingStreaming}
        >
          <Typography.Paragraph
            style={{ marginBottom: 0, whiteSpace: "pre-wrap" }}
            type="secondary"
          >
            {turn.thinking}
          </Typography.Paragraph>
        </Think>
      ) : null}
      {tools.length > 0 ? (
        <ThoughtChain
          line
          items={tools.map((tool) => toolToChainItem(tool, streaming))}
        />
      ) : null}
      {turn.message ? (
        <Bubble
          role="ai"
          content={turn.message}
          loading={streaming && turn.messageStreaming}
          streaming={streaming && turn.messageStreaming}
        />
      ) : null}
    </Space>
  );
}

function workerTurnsToChainItems(turns: TurnView[]): ThoughtChainItemType[] {
  return turns.map((wt) => ({
    key: wt.key,
    title: formatTurnLabel(wt.turn, wt.summary) || `子 Agent 第 ${wt.turn} 轮`,
    icon: <RobotOutlined />,
    status: "success" as const,
    collapsible: true,
    content: renderTurnBody(wt, false, false),
  }));
}

function toolToChainItem(
  tool: ToolView,
  running?: boolean,
): ThoughtChainItemType {
  const displayText = tool.liveOutput || tool.preview;
  const { lang, content } = formatToolMessage(displayText);
  const isStreaming =
    running && tool.status === "loading" && Boolean(tool.outputStreaming);
  const hasWorkers = (tool.workerTurns?.length ?? 0) > 0;
  let itemContent: ReactNode = null;
  if (hasWorkers) {
    itemContent = (
      <div
        style={{
          marginTop: 8,
          paddingLeft: 8,
          borderLeft: "2px solid rgba(0,0,0,0.06)",
        }}
      >
        <ThoughtChain line items={workerTurnsToChainItems(tool.workerTurns!)} />
      </div>
    );
  } else if (content) {
    itemContent = (
      <>
        <CodeHighlighter lang={lang} header={false}>
          {content}
        </CodeHighlighter>
        {isStreaming ? (
          <Typography.Text type="secondary" style={{ fontSize: 12 }}>
            输出流式更新中…
          </Typography.Text>
        ) : null}
      </>
    );
  }
  const footer = tool.elapsedMs != null ? `${tool.elapsedMs} ms` : undefined;
  const description =
    tool.status !== "loading" &&
    tool.preview &&
    (tool.preview.length ?? 0) <= 120
      ? tool.preview
      : undefined;
  return {
    key: tool.toolCallId,
    title: toolTitle(tool),
    icon: tool.serverName ? <ApiOutlined /> : <ToolOutlined />,
    status:
      tool.status === "loading"
        ? ("loading" as const)
        : tool.status === "error"
          ? ("error" as const)
          : tool.status === "abort"
            ? ("abort" as const)
            : ("success" as const),
    blink: running && tool.status === "loading",
    collapsible: Boolean(itemContent) || isStreaming,
    description,
    content: itemContent ?? undefined,
    footer,
  };
}

function subagentProgressLine(snap: SubagentView): string {
  const line = formatAgentProgress(snap.progress, snap.status);
  if (line) return line;
  return snap.description || snap.id;
}

function turnHeader(turn: TurnView) {
  return (
    <Space size="small">
      <Tag color={turn.scope === "worker" ? "purple" : "blue"}>
        {turn.scope === "worker" ? "子 Agent" : "主 Agent"}
      </Tag>
      <span>{formatTurnLabel(turn.turn, turn.summary)}</span>
    </Space>
  );
}

export default function RunActivityPanel({
  state,
  running,
  compact,
}: RunActivityPanelProps) {
  const { turns, result, subagents } = state;
  const visibleTurns = compact && turns.length ? [turns[turns.length - 1]!] : turns;
  const latestKey = visibleTurns.at(-1)?.key;
  const subagentList = Object.values(subagents ?? {});
  const collapseItems = useMemo(
    () =>
      visibleTurns.map((turn) => ({
        key: turn.key,
        label: turnHeader(turn),
        children: renderTurnBody(turn, turn.key === latestKey, running),
      })),
    [visibleTurns, latestKey, running],
  );
  if (!visibleTurns.length && !result?.output && !subagentList.length) {
    if (running) {
      return (
        <Welcome
          icon={<RobotOutlined />}
          title="Agent 运行中"
          description={state.statusLabel || "正在等待第一轮输出，请稍候…"}
        />
      );
    }
    return (
      <Empty
        image={Empty.PRESENTED_IMAGE_SIMPLE}
        description="暂无活动"
      />
    );
  }
  return (
    <>
      {subagentList.length > 0 ? (
        <ThoughtChain
          line
          style={{ marginBottom: 12 }}
          items={subagentList.map((snap) => ({
            key: snap.id,
            title: snap.description || `子 Agent ${snap.id.slice(0, 8)}`,
            icon: <RobotOutlined />,
            status:
              snap.status === "running"
                ? ("loading" as const)
                : ("success" as const),
            description: subagentProgressLine(snap),
            blink: running && snap.status === "running",
          }))}
        />
      ) : null}
      {collapseItems.length > 0 ? (
        <Collapse
          items={collapseItems}
          defaultActiveKey={latestKey ? [latestKey] : []}
        />
      ) : null}
      {result?.output &&
      !visibleTurns.some((t) => (t.message ?? "").includes(result.output!)) ? (
        <div style={{ marginTop: 16 }}>
          <Bubble role="ai" content={result.output} />
        </div>
      ) : null}
    </>
  );
}
