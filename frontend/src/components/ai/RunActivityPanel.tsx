import { useEffect, useMemo, useState } from "react";
import type { ReactNode } from "react";
import { ApiOutlined, RobotOutlined, ToolOutlined } from "@ant-design/icons";
import { Button, Collapse, Empty, Space, Tag, Typography, theme } from "antd";
import {
  CodeHighlighter,
  Think,
  ThoughtChain,
  Welcome,
} from "@ant-design/x";
import MarkdownView from "@/components/docs/MarkdownView";
import type { ThoughtChainItemType } from "@ant-design/x";
import type {
  RunViewState,
  SubagentView,
  ToolView,
  TurnView,
} from "@/types/runView";
import { formatAgentProgress, deriveTurnTitle } from "@/utils/agentProgress";
import { getToolLog } from "@/api/runView";
import { MATRIX_LAYOUT } from "@/theme/layout";

export interface RunActivityPanelProps {
  state: RunViewState;
  running?: boolean;
  /** 紧凑模式：仅展示最后一轮与当前工具链。 */
  compact?: boolean;
  projectId?: string;
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
  projectId?: string,
  runId?: string,
  /** 紧凑模式：正文已在对话气泡中展示，此处只保留思考/工具链，避免叠层重复。 */
  hideMessage?: boolean,
) {
  const streaming = running && isLatest;
  const tools = turn.tools ?? [];
  const messageBody = hideMessage ? null : turnMessageBody(turn);
  return (
    <Space orientation="vertical" size="middle" style={{ width: "100%" }}>
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
          items={tools.map((tool) =>
            toolToChainItem(tool, streaming, projectId, runId),
          )}
        />
      ) : null}
      {messageBody ? (
        <MarkdownView
          content={messageBody}
          variant="chat"
          streaming={streaming && turn.messageStreaming}
        />
      ) : null}
    </Space>
  );
}

function workerTurnStatus(turn: TurnView): ThoughtChainItemType["status"] {
  const tools = turn.tools ?? [];
  if (tools.some((t) => t.status === "loading")) return "loading";
  if (tools.some((t) => t.status === "error")) return "error";
  return "success";
}

function workerTurnsToChainItems(
  turns: TurnView[],
  running?: boolean,
  projectId?: string,
  runId?: string,
): ThoughtChainItemType[] {
  return turns.map((wt) => ({
    key: wt.key,
    title: deriveTurnTitle(wt) || "Worker",
    icon: <RobotOutlined />,
    status: workerTurnStatus(wt),
    blink: running && workerTurnStatus(wt) === "loading",
    collapsible: true,
    content: renderTurnBody(wt, false, running, projectId, runId),
  }));
}

function ToolOutputBlock({
  tool,
  lang,
  content,
  isStreaming,
  projectId,
  runId,
}: {
  tool: ToolView;
  lang: string;
  content: string;
  isStreaming: boolean;
  projectId?: string;
  runId?: string;
}) {
  const { token } = theme.useToken();
  const [fullLog, setFullLog] = useState("");
  const [loadingLog, setLoadingLog] = useState(false);
  const [logError, setLogError] = useState("");
  const canLoadLog = Boolean(projectId && runId && tool.logUrl);
  const shown = fullLog || content;

  async function loadFullLog() {
    if (!projectId || !runId || loadingLog) return;
    setLoadingLog(true);
    setLogError("");
    try {
      const res = await getToolLog(projectId, runId, tool.toolCallId);
      setFullLog(res.content);
    } catch (e) {
      setLogError(e instanceof Error ? e.message : "日志加载失败");
    } finally {
      setLoadingLog(false);
    }
  }

  const textBlock =
    lang === "text" ? (
      <Typography.Paragraph
        style={{
          marginBottom: 0,
          whiteSpace: "pre-wrap",
          fontSize: token.fontSizeSM,
        }}
        type="secondary"
      >
        {shown}
      </Typography.Paragraph>
    ) : (
      <div
        style={{
          maxHeight: MATRIX_LAYOUT.toolOutputMaxHeight,
          overflow: "auto",
        }}
      >
        <CodeHighlighter lang={lang} header={false}>
          {shown}
        </CodeHighlighter>
      </div>
    );

  return (
    <Space orientation="vertical" size={token.marginXXS} style={{ width: "100%" }}>
      {textBlock}
      {isStreaming ? (
        <Typography.Text type="secondary" style={{ fontSize: token.fontSizeSM }}>
          输出流式更新中…
        </Typography.Text>
      ) : null}
      {canLoadLog ? (
        <Button
          type="link"
          size="small"
          loading={loadingLog}
          onClick={loadFullLog}
          style={{ paddingInline: 0 }}
        >
          {fullLog ? "已加载完整日志" : "查看完整日志"}
        </Button>
      ) : null}
      {logError ? (
        <Typography.Text type="danger" style={{ fontSize: token.fontSizeSM }}>
          {logError}
        </Typography.Text>
      ) : null}
    </Space>
  );
}

function WorkerTurnsNest({
  turns,
  running,
  projectId,
  runId,
}: {
  turns: TurnView[];
  running?: boolean;
  projectId?: string;
  runId?: string;
}) {
  const { token } = theme.useToken();
  return (
    <div
      style={{
        marginTop: token.marginXS,
        paddingLeft: token.paddingXS,
        borderLeft: `2px solid ${token.colorBorderSecondary}`,
      }}
    >
      <ThoughtChain
        line
        items={workerTurnsToChainItems(turns, running, projectId, runId)}
      />
    </div>
  );
}

function toolToChainItem(
  tool: ToolView,
  running?: boolean,
  projectId?: string,
  runId?: string,
): ThoughtChainItemType {
  const displayText = tool.liveOutput || tool.preview;
  const { lang, content } = formatToolMessage(displayText);
  const isStreaming =
    running && tool.status === "loading" && Boolean(tool.outputStreaming);
  const hasWorkers = (tool.workerTurns?.length ?? 0) > 0;
  const isAgentTool = tool.toolCallName === "agent";
  let itemContent: ReactNode = null;
  if (hasWorkers) {
    itemContent = (
      <WorkerTurnsNest
        turns={tool.workerTurns!}
        running={running}
        projectId={projectId}
        runId={runId}
      />
    );
  } else if (content) {
    itemContent = (
      <ToolOutputBlock
        tool={tool}
        lang={lang}
        content={content}
        isStreaming={Boolean(isStreaming)}
        projectId={projectId}
        runId={runId}
      />
    );
  } else if (isAgentTool && tool.status === "loading" && !hasWorkers) {
    itemContent = (
      <Typography.Text
        type="secondary"
        style={{ fontSize: "var(--ant-font-size-sm)" }}
      >
        Worker 启动中…
      </Typography.Text>
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
    collapsible: Boolean(itemContent) || isStreaming || isAgentTool,
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

function turnHasActiveTools(turn: TurnView): boolean {
  const visit = (tools: ToolView[]): boolean =>
    tools.some(
      (tool) =>
        tool.status === "loading" ||
        (tool.workerTurns ?? []).some((wt) =>
          (wt.tools ?? []).some((inner) => inner.status === "loading"),
        ),
    );
  return visit(turn.tools ?? []);
}

function selectCompactTurns(
  turns: TurnView[],
  subagents: Record<string, SubagentView> | undefined,
): TurnView[] {
  if (!turns.length) return turns;
  const keys = new Set<string>();
  const picked: TurnView[] = [];
  const push = (turn: TurnView) => {
    if (!keys.has(turn.key)) {
      keys.add(turn.key);
      picked.push(turn);
    }
  };
  push(turns[turns.length - 1]!);
  for (const snap of Object.values(subagents ?? {})) {
    if (!snap.parent_tool_use_id) continue;
    for (const turn of turns) {
      if (
        (turn.tools ?? []).some(
          (tool) => tool.toolCallId === snap.parent_tool_use_id,
        )
      ) {
        push(turn);
      }
    }
  }
  for (const turn of turns) {
    if (turnHasActiveTools(turn)) push(turn);
  }
  return picked.sort((a, b) => a.turn - b.turn);
}

function turnHeader(turn: TurnView) {
  return (
    <Space size="small">
      <Tag color={turn.scope === "worker" ? "purple" : "blue"}>
        {turn.scope === "worker" ? "Worker" : "协调者"}
      </Tag>
      <Typography.Text>{deriveTurnTitle(turn)}</Typography.Text>
    </Space>
  );
}

/** 折叠标题已展示首行摘要时，正文省略重复首行。 */
function turnMessageBody(turn: TurnView): string | null {
  const msg = (turn.message ?? "").trim();
  if (!msg) return null;
  const title = deriveTurnTitle(turn).trim();
  if (!msg.includes("\n")) {
    return msg === title ? null : msg;
  }
  const firstLine = msg.split(/\r?\n/)[0]?.trim() ?? "";
  if (!firstLine || !title.startsWith(firstLine)) return msg;
  const rest = msg.slice(firstLine.length).replace(/^\s*\r?\n/, "").trim();
  return rest || null;
}

function turnHasPanelBody(turn: TurnView): boolean {
  return Boolean(turn.thinking) || (turn.tools?.length ?? 0) > 0;
}

/** 紧凑对话场景：是否值得单独占用活动面板（有思考/工具/子代理，而非仅气泡正文）。 */
export function runViewHasActivityPanel(
  state: RunViewState,
  compact?: boolean,
): boolean {
  if (Object.keys(state.subagents ?? {}).length > 0) return true;
  if (!compact && state.result?.output) return true;
  const turns = compact
    ? selectCompactTurns(state.turns, state.subagents)
    : state.turns;
  if (!turns.length) return true;
  if (!compact) return true;
  return turns.some(turnHasPanelBody);
}

export default function RunActivityPanel({
  state,
  running,
  compact,
  projectId,
}: RunActivityPanelProps) {
  const { turns, result, subagents } = state;
  const visibleTurns = useMemo(
    () => (compact ? selectCompactTurns(turns, subagents) : turns),
    [compact, turns, subagents],
  );
  const latestKey = visibleTurns.at(-1)?.key;
  const subagentList = useMemo(
    () =>
      Object.values(subagents ?? {}).sort(
        (a, b) => (a.created_at ?? 0) - (b.created_at ?? 0),
      ),
    [subagents],
  );
  const [activeKeys, setActiveKeys] = useState<string[]>([]);
  useEffect(() => {
    if (latestKey) setActiveKeys([latestKey]);
  }, [latestKey]);
  const collapseItems = useMemo(
    () =>
      visibleTurns
        .filter((turn) => !compact || turnHasPanelBody(turn))
        .map((turn) => ({
          key: turn.key,
          label: turnHeader(turn),
          children: renderTurnBody(
            turn,
            turn.key === latestKey,
            running,
            projectId,
            state.runId,
            compact,
          ),
        })),
    [visibleTurns, latestKey, running, projectId, state.runId, compact],
  );
  const subagentItems = useMemo(
    () =>
      subagentList.map((snap) => ({
        key: snap.id,
        title: snap.description || `Worker ${snap.id.slice(0, 8)}`,
        icon: <RobotOutlined />,
        status:
          snap.status === "running"
            ? ("loading" as const)
            : snap.status === "failed"
              ? ("error" as const)
              : ("success" as const),
        description: subagentProgressLine(snap),
        blink: running && snap.status === "running",
      })),
    [subagentList, running],
  );
  if (
    compact &&
    !collapseItems.length &&
    !subagentList.length &&
    visibleTurns.length > 0
  ) {
    return null;
  }
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
    <div aria-live="polite">
      {subagentList.length > 0 ? (
        <ThoughtChain
          line
          style={{ marginBottom: 12 }}
          items={subagentItems}
        />
      ) : null}
      {collapseItems.length > 0 ? (
        <Collapse
          style={{ width: "100%" }}
          styles={{ body: { paddingBlock: 12, paddingInline: 4 } }}
          items={collapseItems}
          activeKey={activeKeys}
          onChange={(keys) =>
            setActiveKeys(Array.isArray(keys) ? keys.map(String) : [String(keys)])
          }
        />
      ) : null}
      {result?.output &&
      !compact &&
      !visibleTurns.some((t) => (t.message ?? "").includes(result.output!)) ? (
        <div style={{ marginTop: 16, width: "100%" }}>
          <MarkdownView content={result.output} variant="chat" />
        </div>
      ) : null}
    </div>
  );
}
