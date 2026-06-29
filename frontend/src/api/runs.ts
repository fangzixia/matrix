/**
 * Run / 任务 API（内部执行单元，UI 层称「任务」）。
 */
import { api, HttpError } from "./client";

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
  output?: string;
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

export function getRun(projectId: string, runId: string) {
  return api<Run>(`/api/projects/${projectId}/runs/${runId}`);
}

export function listRunSteps(projectId: string, runId: string) {
  return api<{ steps: RunStep[] }>(
    `/api/projects/${projectId}/runs/${runId}/steps`,
  );
}

export function cancelRun(projectId: string, runId: string) {
  return api<{ ok: boolean }>(
    `/api/projects/${projectId}/runs/${runId}/cancel`,
    { method: "POST" },
  );
}

export async function mergeRun(projectId: string, runId: string) {
  try {
    return await api<Run>(`/api/projects/${projectId}/runs/${runId}/merge`, {
      method: "POST",
    });
  } catch (e) {
    if (e instanceof HttpError) {
      const details = e.details as { conflicts?: string[] } | undefined;
      const err = new Error(e.message || "合并失败") as Error & {
        conflicts?: string[];
      };
      err.conflicts = details?.conflicts;
      throw err;
    }
    throw e;
  }
}

export function discardRun(projectId: string, runId: string) {
  return api<Run>(`/api/projects/${projectId}/runs/${runId}/discard`, {
    method: "POST",
  });
}
