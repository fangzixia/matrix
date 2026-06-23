import type { RunEvent } from "@/api/runs";
import type {
  RunActivityResult,
  RunActivityState,
  RunActivityTurn,
  RunToolStep,
  StreamMessage,
  ToolStepStatus,
} from "@/types/runStream";

function mapToolStatus(status?: string): ToolStepStatus {
  if (status === "started") return "loading";
  if (status === "failed") return "error";
  if (status === "completed") return "success";
  if (status === "success") return "success";
  return "loading";
}

function findToolInTurn(
  turn: RunActivityTurn,
  toolUseId: string,
): RunToolStep | null {
  return turn.tools.find((t) => t.key === toolUseId) ?? null;
}

function findTool(
  turns: RunActivityTurn[],
  toolUseId: string,
): RunToolStep | null {
  for (const turn of turns) {
    const found = findToolInTurn(turn, toolUseId);
    if (found) return found;
  }
  return null;
}

function workerMapKey(msg: StreamMessage): string | null {
  if (!msg.parent_tool_use_id || !msg.agent_id) return null;
  return `${msg.parent_tool_use_id}:${msg.agent_id}`;
}

/** 合并 DB 事件与实时 SSE，按 uuid / event.id 去重。 */
export function mergeStreamSources(
  events: RunEvent[],
  liveMessages: StreamMessage[],
): StreamMessage[] {
  const seen = new Set<string>();
  const result: StreamMessage[] = [];
  for (const e of events) {
    if (!e.payload) continue;
    try {
      const msg = JSON.parse(e.payload) as StreamMessage;
      const key = msg.uuid || e.id;
      if (seen.has(key)) continue;
      seen.add(key);
      result.push(msg);
    } catch {
      /* ignore malformed payload */
    }
  }
  for (const msg of liveMessages) {
    const key = msg.uuid;
    if (!key || seen.has(key)) continue;
    seen.add(key);
    result.push(msg);
  }
  return result;
}

/** 将事件流解析为运行过程视图模型。 */
export function parseRunActivity(messages: StreamMessage[]): RunActivityState {
  const state: RunActivityState = { turns: [] };
  let coordinatorTurn: RunActivityTurn | null = null;
  const workerCurrentTurn = new Map<string, RunActivityTurn>();
  function ensureCoordinatorTurn(
    turnNum: number,
    summary?: string,
  ): RunActivityTurn {
    if (coordinatorTurn && coordinatorTurn.turn === turnNum) {
      if (summary) coordinatorTurn.summary = summary;
      return coordinatorTurn;
    }
    const turn: RunActivityTurn = {
      key: `coord-turn-${turnNum}`,
      turn: turnNum,
      scope: "coordinator",
      summary,
      thinking: "",
      thinkingStreaming: false,
      message: "",
      messageStreaming: false,
      tools: [],
    };
    state.turns.push(turn);
    coordinatorTurn = turn;
    return turn;
  }
  function ensureWorkerTurn(
    msg: StreamMessage,
    turnNum: number,
    summary?: string,
  ): RunActivityTurn {
    const parentId = msg.parent_tool_use_id!;
    const tool = findTool(state.turns, parentId);
    if (!tool) {
      const fallback = ensureCoordinatorTurn(
        Math.max(coordinatorTurn?.turn ?? 0, 1),
      );
      return fallback;
    }
    if (!tool.workerTurns) tool.workerTurns = [];
    const wk = workerMapKey(msg)!;
    const existing = workerCurrentTurn.get(wk);
    if (existing && existing.turn === turnNum) {
      if (summary) existing.summary = summary;
      return existing;
    }
    const turn: RunActivityTurn = {
      key: `worker-${msg.agent_id}-turn-${turnNum}`,
      turn: turnNum,
      scope: "worker",
      agentId: msg.agent_id,
      parentToolUseId: parentId,
      summary,
      thinking: "",
      thinkingStreaming: false,
      message: "",
      messageStreaming: false,
      tools: [],
    };
    tool.workerTurns.push(turn);
    workerCurrentTurn.set(wk, turn);
    return turn;
  }
  function activeTurn(msg: StreamMessage): RunActivityTurn {
    if (msg.scope === "worker") {
      const wk = workerMapKey(msg);
      if (wk && workerCurrentTurn.has(wk)) return workerCurrentTurn.get(wk)!;
      return ensureWorkerTurn(msg, 1);
    }
    if (!coordinatorTurn) return ensureCoordinatorTurn(1);
    return coordinatorTurn;
  }
  function upsertTool(
    turn: RunActivityTurn,
    msg: StreamMessage,
    data: NonNullable<StreamMessage["data"]>,
  ) {
    const toolUseId = msg.tool_use_id;
    if (!toolUseId) return;
    let tool = findToolInTurn(turn, toolUseId);
    const status = mapToolStatus(data.status);
    const toolName = data.tool_name || tool?.toolName || "tool";
    if (!tool) {
      tool = {
        key: toolUseId,
        toolName,
        status,
        serverName: data.server_name,
        message: data.message,
        elapsedMs: data.elapsed_time_ms,
      };
      turn.tools.push(tool);
    } else {
      tool.status = status;
      tool.toolName = toolName;
      if (data.server_name) tool.serverName = data.server_name;
      if (data.message !== undefined) tool.message = data.message;
      if (data.elapsed_time_ms !== undefined)
        tool.elapsedMs = data.elapsed_time_ms;
    }
  }
  for (const msg of messages) {
    if (msg.type === "progress" && msg.data) {
      const data = msg.data;
      if (data.type === "turn_progress") {
        const turnNum = data.turn ?? 1;
        if (msg.scope === "worker") {
          ensureWorkerTurn(msg, turnNum, data.summary);
        } else {
          ensureCoordinatorTurn(turnNum, data.summary);
        }
        continue;
      }
      if (data.type === "tool_progress" || data.type === "mcp_progress") {
        upsertTool(activeTurn(msg), msg, data);
        continue;
      }
    }
    if (msg.type === "stream_event" && msg.event) {
      const turn = activeTurn(msg);
      const ev = msg.event;
      if (ev.type === "message_start") {
        turn.messageStreaming = true;
        turn.thinkingStreaming = true;
        continue;
      }
      if (ev.type === "message_stop") {
        turn.messageStreaming = false;
        turn.thinkingStreaming = false;
        continue;
      }
      if (ev.type === "content_block_delta" && ev.delta) {
        if (ev.delta.type === "thinking_delta" && ev.delta.thinking) {
          turn.thinking += ev.delta.thinking;
        }
        if (ev.delta.type === "text_delta" && ev.delta.text) {
          turn.message += ev.delta.text;
        }
        continue;
      }
    }
    if (msg.type === "assistant" && msg.message?.content) {
      const turn = activeTurn(msg);
      let thinking = "";
      let text = "";
      for (const block of msg.message.content) {
        if (block.type === "thinking" && block.thinking)
          thinking += block.thinking;
        if (block.type === "text" && block.text) text += block.text;
      }
      if (thinking) turn.thinking = thinking;
      if (text) turn.message = text;
      turn.thinkingStreaming = false;
      turn.messageStreaming = false;
      continue;
    }
    if (msg.type === "result") {
      const result: RunActivityResult = {
        subtype: msg.subtype,
        output: msg.output,
        isError: msg.is_error,
        error: msg.error,
        numTurns: msg.num_turns,
        durationMs: msg.duration_ms,
        stopReason: msg.stop_reason,
      };
      if (msg.output && !result.isError) {
        state.result = result;
      } else if (
        msg.is_error ||
        msg.subtype === "error" ||
        msg.subtype === "error_max_turns"
      ) {
        state.result = result;
      } else if (!state.result) {
        state.result = result;
      }
    }
  }
  return state;
}

/** 从 RunEvent 与实时消息构建活动状态。 */
export function buildRunActivityState(
  events: RunEvent[],
  liveMessages: StreamMessage[],
): RunActivityState {
  return parseRunActivity(mergeStreamSources(events, liveMessages));
}
