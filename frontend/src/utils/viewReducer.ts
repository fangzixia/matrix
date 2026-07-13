import type {
  AguiStreamEvent,
  JobRunFinishedValue,
  LoggedEvent,
  RunFinishedResult,
  RunViewState,
  SubagentView,
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

function resultFromRunFinished(result?: RunFinishedResult) {
  const output = typeof result?.output === "string" ? result.output : "";
  const error = typeof result?.error === "string" ? result.error : undefined;
  const status =
    typeof result?.status === "string" ? result.status : "succeeded";
  return { output, error, status };
}

/** 将 AG-UI 扁平事件（LoggedEvent）应用到 RunViewState。 */
export function applyAguiEvent(
  state: RunViewState | null,
  logged: LoggedEvent,
): RunViewState {
  const ev = logged.event;
  const jobId = logged.jobId;
  const seq = logged.seq;
  const base = state ?? emptyState(jobId);

  switch (ev.type) {
    case "STATE_SNAPSHOT": {
      const snap = ev.snapshot as RunViewState | undefined;
      if (!snap) return { ...base, seq };
      return normalizeState({ ...snap, runId: jobId, seq });
    }
    case "TEXT_MESSAGE_CONTENT":
      return {
        ...base,
        seq,
        replyText: base.replyText + (ev.delta ?? ""),
      };
    case "ACTIVITY_SNAPSHOT":
      if (ev.activityType === "subagent" && ev.content) {
        const content = ev.content as unknown as SubagentView;
        if (content.id) {
          return {
            ...base,
            seq,
            subagents: { ...base.subagents, [content.id]: content },
          };
        }
      }
      return { ...base, seq };
    case "RUN_STARTED":
      return { ...base, seq, status: "running" };
    case "RUN_FINISHED": {
      const { output, error, status } = resultFromRunFinished(
        ev.result as RunFinishedResult | undefined,
      );
      return {
        ...base,
        seq,
        status,
        replyText: output.trim() || base.replyText,
        error,
        result: {
          output,
          isError: status === "failed" || status === "cancelled",
          error,
        },
      };
    }
    case "RUN_ERROR":
      return {
        ...base,
        seq,
        status: "failed",
        error: ev.message,
      };
    case "CUSTOM":
      if (ev.name === "job_run_finished") {
        const val = (ev.value ?? {}) as JobRunFinishedValue;
        const status = val.status ?? base.status;
        const output = val.output ?? "";
        const error = val.error;
        return {
          ...base,
          seq,
          status,
          replyText: output.trim() || base.replyText,
          error,
          result: {
            output,
            isError: status === "failed" || status === "cancelled",
            error,
          },
        };
      }
      return { ...base, seq };
    default:
      return { ...base, seq };
  }
}

/** 判断 LoggedEvent 是否表示 Matrix Job 终态。 */
export function isJobTerminalEvent(logged: LoggedEvent): boolean {
  const ev = logged.event;
  if (ev.type === "CUSTOM" && ev.name === "job_run_finished") {
    const status = (ev.value as JobRunFinishedValue | undefined)?.status;
    return status === "succeeded" || status === "failed" || status === "cancelled";
  }
  return false;
}

/** 从 Job 终态事件提取终态载荷。 */
export function jobTerminalFromEvent(logged: LoggedEvent): {
  status: string;
  output?: string;
  error?: string;
} | null {
  if (!isJobTerminalEvent(logged)) return null;
  const val = (logged.event.value ?? {}) as JobRunFinishedValue;
  return {
    status: val.status ?? "failed",
    output: val.output,
    error: val.error,
  };
}

/** @deprecated 旧 ViewEnvelope 归约，内部转发 applyAguiEvent。 */
export function applyEnvelope(
  state: RunViewState | null,
  env: ViewEnvelope,
): RunViewState {
  const flat: AguiStreamEvent = {
    type: env.type,
    ...(typeof env.payload === "object" && env.payload !== null
      ? (env.payload as AguiStreamEvent)
      : {}),
  };
  if (env.type === "STATE_SNAPSHOT") {
    flat.snapshot = env.payload as RunViewState;
  }
  return applyAguiEvent(state, {
    jobId: env.runId,
    seq: env.seq,
    timestamp: env.timestamp,
    event: flat,
  });
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
  if (/git clone 失败|git clone 超时|source fetch failed/i.test(msg)) {
    return "源码获取失败，请检查仓库地址、分支名称与 Git 访问权限后重试。";
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
