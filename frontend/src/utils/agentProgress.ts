/** Worker / Run 活动进度文案 */

import type { ToolView, TurnView } from "@/types/runView";

function isActiveToolActivity(lastActivity: string, currentTool: string): boolean {
  if (!currentTool) return false;
  return (
    lastActivity.startsWith("正在调用 ") ||
    lastActivity.includes("输出中") ||
    lastActivity.includes("执行失败")
  );
}

/** 将 progress 快照格式化为单行活动描述。 */
export function formatAgentProgress(
  progress: Record<string, unknown> | undefined,
  status?: string,
): string {
  if (!progress) return "";
  const currentTool =
    typeof progress.current_tool === "string" ? progress.current_tool : "";
  const lastActivity =
    typeof progress.last_activity === "string" ? progress.last_activity : "";
  const toolUseCount =
    typeof progress.tool_use_count === "number" ? progress.tool_use_count : 0;
  const transition =
    typeof progress.transition === "string" ? progress.transition : "";

  if (status === "completed") return "已完成";
  if (status === "failed") return "执行失败";
  if (status === "stopped") return "已停止";
  if (lastActivity === "已完成") return "已完成";
  if (lastActivity === "整理回复中…") return lastActivity;
  if (lastActivity.endsWith("思考中…") || lastActivity.includes("校验未通过")) {
    return lastActivity;
  }

  if (currentTool && isActiveToolActivity(lastActivity, currentTool)) {
    return lastActivity;
  }

  if (
    transition === "stop_hook_blocking" &&
    !currentTool
  ) {
    return "校验未通过，正在重试…";
  }

  if (toolUseCount > 0 && !currentTool) {
    return `已调用 ${toolUseCount} 个工具`;
  }

  if (lastActivity) return lastActivity;

  return "思考中…";
}

const GENERIC_TURN_SUMMARY = /^第 \d+ 轮( · .+)?$/;
const GENERIC_ACTIVITY_LABELS = new Set(["思考中…", "等待 Worker 完成…"]);

/** 识别无意义的占位 summary。 */
export function isGenericTurnSummary(summary?: string): boolean {
  const s = summary?.trim();
  if (!s) return true;
  if (s.includes("跃迁:")) return true;
  if (GENERIC_ACTIVITY_LABELS.has(s)) return true;
  return GENERIC_TURN_SUMMARY.test(s);
}

function firstLine(text: string): string {
  const trimmed = text.trim();
  if (!trimmed) return "";
  const idx = trimmed.search(/\r?\n/);
  return (idx >= 0 ? trimmed.slice(0, idx) : trimmed).trim();
}

function truncateRunes(text: string, max: number): string {
  const runes = [...text];
  if (runes.length <= max) return text;
  return runes.slice(0, max).join("") + "…";
}

function stringArg(args: Record<string, unknown>, key: string): string {
  const v = args[key];
  if (v == null) return "";
  return String(v).trim();
}

function formatPathIntent(toolName: string, path: string): string {
  const p = truncateRunes(path, 60);
  switch (toolName) {
    case "read_file":
      return `读取 ${p}`;
    case "write_file":
      return `写入 ${p}`;
    case "list_dir":
      return `列出 ${p}`;
    case "str_replace_editor":
      return `编辑 ${p}`;
    default:
      return p;
  }
}

function intentFromLiveOutput(liveOutput: string): string {
  let line = firstLine(liveOutput);
  if (!line) return "";
  line = line.replace(/…$/, "").trim();
  if (!line) return "";

  if (
    line.startsWith("读取 ") ||
    line.startsWith("写入 ") ||
    line.startsWith("编辑 ") ||
    line.startsWith("列出 ") ||
    line.startsWith("获取 ")
  ) {
    return line;
  }
  if (line.startsWith("搜索:") || line.startsWith("搜索：")) {
    return line.replace(/^搜索[:：]\s*/, "").trim();
  }
  if (line.startsWith("子 Agent:") || line.startsWith("子 Agent：")) {
    return line.replace(/^子 Agent[:：]\s*/, "").trim();
  }
  if (line.startsWith("Worker:") || line.startsWith("Worker：")) {
    return line.replace(/^Worker[:：]\s*/, "").trim();
  }
  if (
    line.startsWith("$ ") ||
    line.startsWith("PS> ") ||
    line.startsWith("grep ") ||
    line.startsWith("glob ") ||
    line.startsWith("MCP ")
  ) {
    return truncateRunes(line, 80);
  }
  return "";
}

function intentFromPartialJSON(name: string, preview: string): string {
  const keys = [
    "description",
    "target_path",
    "path",
    "file_path",
    "pattern",
    "query",
    "command",
    "url",
    "prompt",
  ] as const;
  for (const key of keys) {
    const re = new RegExp(`"${key}"\\s*:\\s*"([^"]*)"`);
    const m = preview.match(re);
    if (!m?.[1]) continue;
    const val = m[1];
    switch (key) {
      case "description":
        if (name === "agent") {
          return `委派 Worker「${truncateRunes(val, 60)}」`;
        }
        return truncateRunes(val, 80);
      case "query":
      case "prompt":
        return truncateRunes(val, 80);
      case "target_path":
      case "path":
      case "file_path":
        return formatPathIntent(name, val);
      case "pattern":
        return `${name} ${truncateRunes(val, 60)}`;
      case "command":
        return truncateRunes(val, 80);
      case "url":
        return `获取 ${truncateRunes(val, 60)}`;
    }
  }
  return "";
}

function intentFromPreview(name: string, preview: string): string {
  if (!preview) return "";
  try {
    const args = JSON.parse(preview) as Record<string, unknown>;
    const desc = stringArg(args, "description");
    if (desc) {
      if (name === "agent") {
        return `委派 Worker「${truncateRunes(desc, 60)}」`;
      }
      return truncateRunes(desc, 80);
    }

    const path = [stringArg(args, "target_path"), stringArg(args, "path"), stringArg(args, "file_path")].find(Boolean);
    if (
      path &&
      ["read_file", "write_file", "list_dir", "str_replace_editor"].includes(name)
    ) {
      return formatPathIntent(name, path);
    }
    if (["grep", "glob"].includes(name)) {
      const pattern = stringArg(args, "pattern");
      if (pattern) return `${name} ${truncateRunes(pattern, 60)}`;
    }
    if (name === "bash" || name === "powershell") {
      const cmd = stringArg(args, "command");
      if (cmd) return truncateRunes(cmd, 80);
    }
    if (name === "web_search") {
      const q = stringArg(args, "query");
      if (q) return `搜索: ${truncateRunes(q, 60)}`;
    }
    if (name === "web_fetch") {
      const u = stringArg(args, "url");
      if (u) return `获取 ${truncateRunes(u, 60)}`;
    }
    if (name === "agent") {
      const prompt = stringArg(args, "prompt");
      if (prompt) return truncateRunes(prompt, 80);
    }
    const query = stringArg(args, "query");
    if (query) return truncateRunes(query, 80);
  } catch {
    return intentFromPartialJSON(name, preview);
  }
  return "";
}

function extractToolIntent(tool: ToolView): string {
  const name = tool.toolCallName || "tool";
  const fromLive = intentFromLiveOutput(tool.liveOutput ?? "");
  if (fromLive) return fromLive;
  const fromPreview = intentFromPreview(name, tool.preview ?? "");
  if (fromPreview) return fromPreview;
  if (name !== "tool") return name;
  return "";
}

/** 从轮次实际活动推导用户可见标题（与后端 DeriveTurnSummary 对齐）。 */
export function deriveTurnTitle(turn: TurnView): string {
  if (turn.summary && !isGenericTurnSummary(turn.summary)) {
    return turn.summary;
  }
  const tools = turn.tools ?? [];
  const parts = tools.map(extractToolIntent).filter(Boolean);
  if (parts.length > 0) {
    return truncateRunes(parts.join(" · "), 120);
  }
  const messageLine = firstLine(turn.message ?? "");
  if (messageLine) return truncateRunes(messageLine, 80);
  return "思考中…";
}
