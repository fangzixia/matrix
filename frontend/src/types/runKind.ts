/** Run / 任务类型枚举 */

export const RunKind = {
  Plan: "plan",
  Implement: "implement",
  Verify: "verify",
  /** 编排模式：同工作区循环 implement → verify，非独立 Harness prompt */
  Build: "build",
  Chat: "chat",
} as const;

export type RunKind = (typeof RunKind)[keyof typeof RunKind];

export const STAGE_KINDS = [
  RunKind.Plan,
  RunKind.Implement,
  RunKind.Verify,
  RunKind.Build,
] as const;

export type StageKind = (typeof STAGE_KINDS)[number];

const runKindSet = new Set<string>(Object.values(RunKind));
const stageKindSet = new Set<string>(STAGE_KINDS);

export function isRunKind(kind: string): kind is RunKind {
  return runKindSet.has(kind);
}

export function isStageKind(kind: string): kind is StageKind {
  return stageKindSet.has(kind);
}

export function requiresPlanFile(kind: RunKind | StageKind): boolean {
  return (
    kind === RunKind.Implement ||
    kind === RunKind.Verify ||
    kind === RunKind.Build
  );
}

export function requiresApprovedPlan(kind: RunKind | StageKind): boolean {
  return requiresPlanFile(kind);
}

export function stageKindFromPath(pathname: string): StageKind | null {
  const match = pathname.match(
    /\/projects\/[^/]+\/(plan|implement|verify|build)(?:\/|$)/,
  );
  if (!match || !isStageKind(match[1])) return null;
  return match[1];
}
