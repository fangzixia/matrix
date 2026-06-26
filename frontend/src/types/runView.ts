/** Run 视图类型，对齐 internal/modules/run/view */

export type StreamMode = "chat" | "detail";

export type ViewEventType =
  | "RUN_STARTED"
  | "RUN_FINISHED"
  | "RUN_ERROR"
  | "TEXT_MESSAGE_START"
  | "TEXT_MESSAGE_CONTENT"
  | "TEXT_MESSAGE_END"
  | "REASONING_MESSAGE_CONTENT"
  | "REASONING_MESSAGE_END"
  | "TOOL_CALL_START"
  | "TOOL_CALL_ARGS"
  | "TOOL_CALL_END"
  | "TOOL_CALL_RESULT"
  | "ACTIVITY_SNAPSHOT"
  | "STATE_SNAPSHOT"
  | "STEP_STARTED"
  | "STEP_FINISHED";

export interface ViewEnvelope<T = unknown> {
  type: ViewEventType | string;
  runId: string;
  seq: number;
  timestamp: number;
  payload: T;
}

export interface RunViewState {
  runId: string;
  seq: number;
  status: string;
  phase?: string;
  statusLabel: string;
  replyText: string;
  replyMessageId?: string;
  turns: TurnView[];
  subagents?: Record<string, SubagentView>;
  result?: ResultView;
  error?: string;
}

export interface TurnView {
  key: string;
  turn: number;
  scope: "coordinator" | "worker" | string;
  agentId?: string;
  parentToolUseId?: string;
  summary?: string;
  thinking: string;
  thinkingStreaming: boolean;
  message: string;
  messageStreaming: boolean;
  tools: ToolView[];
}

export interface ToolView {
  toolCallId: string;
  toolCallName: string;
  status: "loading" | "success" | "error" | "abort" | string;
  preview?: string;
  liveOutput?: string;
  outputStreaming?: boolean;
  elapsedMs?: number;
  serverName?: string;
  logUrl?: string;
  workerTurns?: TurnView[];
}

export interface SubagentView {
  id: string;
  description?: string;
  status?: string;
  parent_agent_id?: string;
  parent_tool_use_id?: string;
  progress?: Record<string, unknown>;
  created_at?: number;
  sidechain_path?: string;
  answer_preview?: string;
  turn_count?: number;
}

export interface ResultView {
  subtype?: string;
  output?: string;
  isError?: boolean;
  error?: string;
  numTurns?: number;
  durationMs?: number;
  stopReason?: string;
}

export interface RunFinishedPayload {
  status: string;
  output?: string;
  error?: string;
  mergeStatus?: string;
}

export interface TextDeltaPayload {
  messageId: string;
  delta: string;
}
