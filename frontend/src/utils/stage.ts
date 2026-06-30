export const STAGE_KINDS = ["plan", "implement", "verify", "build"] as const;

export type StageKind = (typeof STAGE_KINDS)[number];

const stageKindSet = new Set<string>(STAGE_KINDS);

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
