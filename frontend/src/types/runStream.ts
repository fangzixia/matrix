/** Agent 流式消息类型，对齐 internal/ai/stream/message.go */

export type StreamScope = "coordinator" | "worker";

export interface ToolProgressData {
  type: "turn_progress" | "tool_progress" | "mcp_progress";
  status?: string;
  turn?: number;
  transition?: string;
  summary?: string;
  tool_name?: string;
  server_name?: string;
  elapsed_time_ms?: number;
  message?: string;
}

export interface BlockDelta {
  type: "text_delta" | "thinking_delta";
  text?: string;
  thinking?: string;
}

export interface StreamEventPayload {
  type: string;
  index?: number;
  delta?: BlockDelta;
  usage?: unknown;
}

export interface ContentBlock {
  type: string;
  text?: string;
  thinking?: string;
}

export interface ToolUseBlock {
  id: string;
  name: string;
  input: string;
}

export interface AssistantPayload {
  role: string;
  content: ContentBlock[];
  tool_calls?: ToolUseBlock[];
  stop_reason?: string;
}

export interface StreamMessage {
  type?: "progress" | "stream_event" | "assistant" | "result" | string;
  session_id?: string;
  uuid?: string;
  scope?: StreamScope;
  agent_id?: string;
  parent_agent_id?: string;
  parent_tool_use_id?: string;
  tool_use_id?: string;
  data?: ToolProgressData;
  event?: StreamEventPayload;
  message?: AssistantPayload;
  subtype?: string;
  stop_reason?: string;
  num_turns?: number;
  duration_ms?: number;
  is_error?: boolean;
  error?: string;
  output?: string;
}

export type ToolStepStatus = "loading" | "success" | "error" | "abort";

export interface RunActivityTurn {
  key: string;
  turn: number;
  scope: StreamScope;
  agentId?: string;
  parentToolUseId?: string;
  summary?: string;
  thinking: string;
  thinkingStreaming: boolean;
  message: string;
  messageStreaming: boolean;
  tools: RunToolStep[];
}

export interface RunToolStep {
  key: string;
  toolName: string;
  status: ToolStepStatus;
  message?: string;
  elapsedMs?: number;
  serverName?: string;
  workerTurns?: RunActivityTurn[];
}

export interface RunActivityResult {
  subtype?: string;
  output?: string;
  isError?: boolean;
  error?: string;
  numTurns?: number;
  durationMs?: number;
  stopReason?: string;
}

export interface RunActivityState {
  turns: RunActivityTurn[];
  result?: RunActivityResult;
}
