/** 子 Agent / Run 活动进度文案（方案 B：活动导向） */

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
  const turn = typeof progress.turn === "number" ? progress.turn : 0;
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
    turn > 0 &&
    !currentTool
  ) {
    return `第 ${turn} 轮 · 校验未通过，正在重试…`;
  }

  if (toolUseCount > 0 && !currentTool) {
    if (turn <= 1) return `已调用 ${toolUseCount} 个工具`;
    return `第 ${turn} 轮 · 已调用 ${toolUseCount} 个工具`;
  }

  if (lastActivity) return lastActivity;

  if (turn <= 1) return "思考中…";
  if (turn > 0) return `第 ${turn} 轮 · 思考中…`;
  return "";
}

/** 轮次标题：优先使用后端 summary，否则回退轮次号。 */
export function formatTurnLabel(
  turn: number,
  summary?: string,
): string {
  if (summary && !summary.includes("跃迁:")) {
    return summary;
  }
  if (turn > 0) return `第 ${turn} 轮`;
  return "进行中";
}
