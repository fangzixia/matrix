import type {
  RunFinishedPayload,
  RunViewState,
  TextDeltaPayload,
  TurnView,
  ViewEnvelope,
} from "@/types/runView";

const emptyState = (runId: string): RunViewState => ({
  runId,
  seq: 0,
  status: "running",
  statusLabel: "",
  replyText: "",
  turns: [],
});

function normalizeTurn(turn: TurnView): TurnView {
  return {
    ...turn,
    thinking: turn.thinking ?? "",
    message: turn.message ?? "",
    tools: (turn.tools ?? []).map((tool) => ({
      ...tool,
      workerTurns: tool.workerTurns?.map(normalizeTurn),
    })),
  };
}

function normalizeState(state: RunViewState): RunViewState {
  return {
    ...state,
    replyText: state.replyText ?? "",
    statusLabel: state.statusLabel ?? "",
    turns: (state.turns ?? []).map(normalizeTurn),
  };
}

/** 规范化后端快照中的 nullable 字段，避免前端访问 null.length 崩溃。 */
export function normalizeRunViewState(state: RunViewState): RunViewState {
  return normalizeState(state);
}

/** 将 SSE 事件应用到 RunViewState。 */
export function applyEnvelope(
  state: RunViewState | null,
  env: ViewEnvelope,
): RunViewState {
  const base = state ?? emptyState(env.runId);

  switch (env.type) {
    case "STATE_SNAPSHOT":
      return normalizeState({ ...(env.payload as RunViewState), seq: env.seq });
    case "TEXT_MESSAGE_CONTENT": {
      const pl = env.payload as TextDeltaPayload;
      return {
        ...base,
        seq: env.seq,
        replyText: base.replyText + (pl.delta ?? ""),
      };
    }
    case "ACTIVITY_SNAPSHOT": {
      const pl = env.payload as {
        subagents?: RunViewState["subagents"];
        statusLabel?: string;
      };
      return {
        ...base,
        seq: env.seq,
        subagents: pl.subagents ?? base.subagents,
        statusLabel: pl.statusLabel ?? base.statusLabel,
      };
    }
    case "RUN_FINISHED": {
      const pl = env.payload as RunFinishedPayload;
      return {
        ...base,
        seq: env.seq,
        status: pl.status,
        replyText: pl.output?.trim() || base.replyText,
        error: pl.error,
        result: {
          output: pl.output,
          isError: pl.status === "failed" || pl.status === "cancelled",
          error: pl.error,
        },
      };
    }
    case "RUN_STARTED": {
      const pl = env.payload as { statusLabel?: string; phase?: string };
      return {
        ...base,
        seq: env.seq,
        status: "running",
        statusLabel: pl.statusLabel ?? base.statusLabel,
        phase: pl.phase,
      };
    }
    default:
      return { ...base, seq: env.seq };
  }
}

/** 从视图状态提取面向用户的最终回复文本。 */
export function extractReplyText(state: RunViewState | null): string {
  if (!state) return "";
  if (state.result?.output && !state.result.isError) {
    return state.result.output.trim();
  }
  if (state.replyText.trim()) {
    return state.replyText.trim();
  }
  for (let i = state.turns.length - 1; i >= 0; i--) {
    const text = state.turns[i]?.message?.trim();
    if (text) return text;
  }
  return "";
}

/** 将后端/模型原始错误转为用户可读文案。 */
export function formatUserRunError(raw: string): string {
  const trimmed = raw.trim();
  if (!trimmed) return "任务执行失败，请稍后重试";

  let msg = trimmed
    .replace(/^loop:\s*模型错误:\s*/i, "")
    .replace(/^llm:\s*/i, "");

  const jsonMatch = msg.match(/服务端返回 \d+:\s*(\{[\s\S]+\})/);
  if (jsonMatch) {
    try {
      const parsed = JSON.parse(jsonMatch[1]) as {
        error?: { message?: string };
      };
      if (parsed.error?.message) msg = parsed.error.message;
    } catch {
      /* keep msg */
    }
  }

  if (/authentication|api key|invalid.*key|\b401\b/i.test(msg)) {
    return "模型 API Key 无效或已过期，请在「管理区域 → 系统配置 → AI 模型」中检查配置。";
  }
  if (/未配置模型|未配置 API Key/i.test(msg)) {
    return msg;
  }
  if (/429|rate limit|too many requests/i.test(msg)) {
    return "模型服务请求过于频繁，请稍后再试。";
  }
  if (/服务端返回 5\d\d|internal server error/i.test(msg)) {
    return "模型服务暂时不可用，请稍后再试。";
  }
  if (msg.length > 200) {
    return "模型调用失败，请检查系统配置中的 AI 模型设置。";
  }
  return msg;
}

/** 从视图状态或 Run 摘要中提取失败原因。 */
export function extractRunFailure(
  state: RunViewState | null,
  runErrorMessage?: string,
): string | null {
  const fromResult =
    state?.result?.isError && state.result.error?.trim()
      ? state.result.error.trim()
      : "";
  const raw = fromResult || state?.error?.trim() || runErrorMessage?.trim() || "";
  if (!raw) return null;
  return formatUserRunError(raw);
}

