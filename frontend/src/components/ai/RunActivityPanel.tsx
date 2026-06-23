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
  RunActivityState,
  RunActivityTurn,
  RunToolStep,
} from "@/types/runStream";

export interface RunActivityPanelProps {
  state: RunActivityState;
  running?: boolean;
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

function toolTitle(tool: RunToolStep): string {
  if (tool.serverName) return `${tool.serverName} / ${tool.toolName}`;
  return tool.toolName;
}

function renderTurnBody(
  turn: RunActivityTurn,
  isLatest: boolean,
  running?: boolean,
) {
  const streaming = running && isLatest;
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
      {turn.tools.length > 0 ? (
        <ThoughtChain
          line
          items={turn.tools.map((tool) => toolToChainItem(tool, streaming))}
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

function workerTurnsToChainItems(
  turns: RunActivityTurn[],
): ThoughtChainItemType[] {
  return turns.map((wt) => ({
    key: wt.key,
    title: wt.summary || `子 Agent 第 ${wt.turn} 轮`,
    icon: <RobotOutlined />,
    status: "success" as const,
    collapsible: true,
    content: renderTurnBody(wt, false, false),
  }));
}

function toolToChainItem(
  tool: RunToolStep,
  running?: boolean,
): ThoughtChainItemType {
  const { lang, content } = formatToolMessage(tool.message);
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
      <CodeHighlighter lang={lang} header={false}>
        {content}
      </CodeHighlighter>
    );
  }
  const footer = tool.elapsedMs != null ? `${tool.elapsedMs} ms` : undefined;
  const description =
    tool.status !== "loading" && tool.message && tool.message.length <= 120
      ? tool.message
      : undefined;
  return {
    key: tool.key,
    title: toolTitle(tool),
    icon: tool.serverName ? <ApiOutlined /> : <ToolOutlined />,
    status: tool.status,
    blink: running && tool.status === "loading",
    collapsible: Boolean(itemContent),
    description,
    content: itemContent ?? undefined,
    footer,
  };
}

function turnHeader(turn: RunActivityTurn) {
  return (
    <Space size="small">
      <Tag color={turn.scope === "worker" ? "purple" : "blue"}>
        {turn.scope === "worker" ? "子 Agent" : "主 Agent"}
      </Tag>
      <span>{turn.summary || `第 ${turn.turn} 轮`}</span>
    </Space>
  );
}

export default function RunActivityPanel({
  state,
  running,
}: RunActivityPanelProps) {
  const { turns, result } = state;
  const latestKey = turns.at(-1)?.key;
  const collapseItems = useMemo(
    () =>
      turns.map((turn) => ({
        key: turn.key,
        label: turnHeader(turn),
        children: renderTurnBody(turn, turn.key === latestKey, running),
      })),
    [turns, latestKey, running],
  );
  if (!turns.length && !result?.output) {
    if (running) {
      return (
        <Welcome
          icon={<RobotOutlined />}
          title="Agent 运行中"
          description="正在等待第一轮输出，请稍候…"
        />
      );
    }
    return (
      <Empty
        image={Empty.PRESENTED_IMAGE_SIMPLE}
        description="等待 Agent 输出…"
      />
    );
  }
  return (
    <>
      {collapseItems.length > 0 ? (
        <Collapse
          items={collapseItems}
          defaultActiveKey={latestKey ? [latestKey] : []}
        />
      ) : null}
      {result?.output &&
      !turns.some((t) => t.message.includes(result.output!)) ? (
        <div style={{ marginTop: 16 }}>
          <Bubble role="ai" content={result.output} />
        </div>
      ) : null}
    </>
  );
}
