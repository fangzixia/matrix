/** Agent 流式消息类型，对齐 internal/ai/stream/message.go */

export type StreamScope = "coordinator" | "worker";

export interface ToolProgressData {
  type:
    | "turn_progress"
    | "tool_progress"
    | "mcp_progress"
    | "tool_output_delta";
  status?: string;
  turn?: number;
  transition?: string;
  summary?: string;
  tool_name?: string;
  server_name?: string;
  elapsed_time_ms?: number;
  message?: string;
  delta?: string;
  output_offset?: number;
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
  snapshot?: AgentSnapshot;
}

export interface ToolActivity {
  tool_name: string;
  status?: string;
  preview?: string;
}

export interface AgentProgress {
  turn?: number;
  transition?: string;
  summary?: string;
  current_tool?: string;
  tool_use_count?: number;
  last_activity?: string;
  input_tokens?: number;
  output_tokens?: number;
  recent_activities?: ToolActivity[];
}

export interface AgentSnapshot {
  id: string;
  description?: string;
  status?: string;
  parent_agent_id?: string;
  parent_tool_use_id?: string;
  progress?: AgentProgress;
  created_at?: number;
  sidechain_path?: string;
  answer_preview?: string;
  turn_count?: number;
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
  liveOutput?: string;
  outputStreaming?: boolean;
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
  subagents?: Record<string, AgentSnapshot>;
  result?: RunActivityResult;
}
