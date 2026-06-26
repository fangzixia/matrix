/** Run / Chat 流式链路诊断日志（浏览器控制台，前缀 [matrix:run]） */

const PREFIX = "[matrix:run]";

function enabled(): boolean {
  if (import.meta.env.DEV) return true;
  try {
    return localStorage.getItem("matrix:run-debug") === "1";
  } catch {
    return false;
  }
}

export function runDebug(event: string, detail?: Record<string, unknown>): void {
  if (!enabled()) return;
  if (detail != null) {
    console.info(PREFIX, event, detail);
  } else {
    console.info(PREFIX, event);
  }
}

export function runDebugWarn(event: string, detail?: Record<string, unknown>): void {
  if (!enabled()) return;
  if (detail != null) {
    console.warn(PREFIX, event, detail);
  } else {
    console.warn(PREFIX, event);
  }
}
