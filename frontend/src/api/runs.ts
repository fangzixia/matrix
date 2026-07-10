/**
 * Run / 任务 API（内部执行单元，UI 层称「任务」）。
 */
import { api } from "./client";
import type { RunKind, StageKind } from "@/types/runKind";

/** AI 任务执行记录 */
export interface Run {
  id: string;
  project_id: string;
  repository_id?: string;
  kind: RunKind;
  status: string;
  title?: string;
  audit_path?: string;
  sandbox_path?: string;
  error_message?: string;
  output?: string;
  started_at?: string;
  finished_at?: string;
  created_at: string;
}

export function listRuns(projectId: string, kind?: RunKind | StageKind) {
  const q = kind ? `?kind=${encodeURIComponent(kind)}` : "";
  return api<{ runs: Run[] }>(`/api/projects/${projectId}/runs${q}`);
}

export function startRun(
  projectId: string,
  message: string,
  kind: RunKind | StageKind = "plan",
  filePath = "",
) {
  return api<Run>(`/api/projects/${projectId}/runs`, {
    method: "POST",
    body: JSON.stringify({
      message,
      kind,
      file_path: filePath,
    }),
  });
}

export function getRun(projectId: string, runId: string) {
  return api<Run>(`/api/projects/${projectId}/runs/${runId}`);
}

export function cancelRun(projectId: string, runId: string) {
  return api<{ ok: boolean }>(
    `/api/projects/${projectId}/runs/${runId}/cancel`,
    { method: "POST" },
  );
}
