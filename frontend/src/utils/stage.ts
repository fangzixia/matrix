export const STAGE_KINDS = ["plan", "implement", "verify", "build"] as const;

export type StageKind = (typeof STAGE_KINDS)[number];

const stageKindSet = new Set<string>(STAGE_KINDS);

const worktreeKindSet = new Set<string>(["implement", "build"]);

export function usesWorktreeKind(kind: string): boolean {
  return worktreeKindSet.has(kind);
}

export function canMergeRun(run: {
  status: string;
  merge_status?: string;
  kind: string;
}): boolean {
  if (run.status !== "succeeded" || run.merge_status !== "pending")
    return false;
  return usesWorktreeKind(run.kind) || run.kind === "pipeline";
}

export function isStageKind(kind: string): kind is StageKind {
  return stageKindSet.has(kind);
}

export function stageKindFromPath(pathname: string): StageKind | null {
  const match = pathname.match(
    /\/projects\/[^/]+\/(plan|implement|verify|build)(?:\/|$)/,
  );
  if (!match || !isStageKind(match[1])) return null;
  return match[1];
}
