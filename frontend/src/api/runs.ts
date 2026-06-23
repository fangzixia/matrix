/**
 * Run / 任务 API（内部执行单元，UI 层称「任务」）。
 */
import { api } from "./client";

/** AI 任务执行记录 */
export interface Run {
  id: string;
  project_id: string;
  repository_id?: string;
  kind: string;
  status: string;
  title?: string;
  audit_path?: string;
  sandbox_path?: string;
  run_branch?: string;
  merge_status?: string;
  error_message?: string;
  started_at?: string;
  finished_at?: string;
  created_at: string;
}

/** Run 内步骤 */
export interface RunStep {
  id: string;
  run_id: string;
  kind: string;
  sequence: number;
  status: string;
  output_summary?: string;
  started_at?: string;
  finished_at?: string;
}

/** Run 事件快照 */
export interface RunEvent {
  id: string;
  run_id: string;
  step_id?: string;
  event_type: string;
  payload?: string;
  created_at: string;
}

export function listRuns(projectId: string, kind?: string) {
  const q = kind ? `?kind=${encodeURIComponent(kind)}` : "";
  return api<{ runs: Run[] }>(`/api/projects/${projectId}/runs${q}`);
}

export function startRun(
  projectId: string,
  message: string,
  kind = "plan",
  filePath = "",
  evalFilePath = "",
  sync = false,
) {
  const q = sync ? "?sync=1" : "";
  return api<Run>(`/api/projects/${projectId}/runs${q}`, {
    method: "POST",
    body: JSON.stringify({
      message,
      kind,
      file_path: filePath,
      eval_file_path: evalFilePath,
    }),
  });
}

export function startPipeline(
  projectId: string,
  message: string,
  filePath: string,
  stages: string[] = ["plan", "build"],
) {
  return api<Run>(`/api/projects/${projectId}/runs`, {
    method: "POST",
    body: JSON.stringify({
      message,
      kind: "pipeline",
      file_path: filePath,
      stages,
    }),
  });
}

export function getRun(projectId: string, runId: string) {
  return api<Run>(`/api/projects/${projectId}/runs/${runId}`);
}

export function listRunSteps(projectId: string, runId: string) {
  return api<{ steps: RunStep[] }>(
    `/api/projects/${projectId}/runs/${runId}/steps`,
  );
}

export function listRunEvents(
  projectId: string,
  runId: string,
  afterId?: string,
) {
  const q = afterId ? `?after_id=${afterId}` : "";
  return api<{ events: RunEvent[] }>(
    `/api/projects/${projectId}/runs/${runId}/events${q}`,
  );
}

export function getRunAudit(projectId: string, runId: string) {
  return api<{ content: string }>(
    `/api/projects/${projectId}/runs/${runId}/audit`,
  );
}

export function cancelRun(projectId: string, runId: string) {
  return api<{ ok: boolean }>(
    `/api/projects/${projectId}/runs/${runId}/cancel`,
    { method: "POST" },
  );
}

export async function mergeRun(projectId: string, runId: string) {
  const res = await fetch(`/api/projects/${projectId}/runs/${runId}/merge`, {
    method: "POST",
    credentials: "include",
  });
  const body = (await res.json().catch(() => ({}))) as Run & {
    error?: string;
    conflicts?: string[];
  };
  if (!res.ok) {
    const err = new Error(body.error || "合并失败") as Error & {
      conflicts?: string[];
    };
    err.conflicts = body.conflicts;
    throw err;
  }
  return body as Run;
}

export function discardRun(projectId: string, runId: string) {
  return api<Run>(`/api/projects/${projectId}/runs/${runId}/discard`, {
    method: "POST",
  });
}
