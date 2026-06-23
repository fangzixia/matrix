/** 从 Git 地址推断项目编码建议值（仅用于 UI 自动建议，不替代用户必填）。 */
export function inferProjectCodeFromGitUrl(gitUrl: string): string {
  let raw = gitUrl.trim();
  if (!raw) return "";
  if (raw.endsWith(".git")) raw = raw.slice(0, -4);
  try {
    const u = new URL(raw.includes("://") ? raw : `https://${raw}`);
    const seg = u.pathname
      .replace(/^\/+|\/+$/g, "")
      .split("/")
      .pop();
    if (seg) return normalizeProjectCode(seg);
  } catch {
    /* ssh/scp 等形式走下方 fallback */
  }
  const colon = raw.lastIndexOf(":");
  if (colon >= 0 && colon < raw.length - 1) {
    const tail =
      raw
        .slice(colon + 1)
        .split("/")
        .pop() || "";
    if (tail) return normalizeProjectCode(tail);
  }
  const slash = raw.lastIndexOf("/");
  if (slash >= 0 && slash < raw.length - 1) {
    return normalizeProjectCode(raw.slice(slash + 1));
  }
  return normalizeProjectCode(raw);
}

export function normalizeProjectCode(code: string): string {
  let out = code
    .toLowerCase()
    .trim()
    .replace(/[^a-z0-9-_.]+/g, "-")
    .replace(/-+/g, "-");
  out = out.replace(/^[-.]+|[-.]+$/g, "");
  if (out.length > 64) out = out.slice(0, 64).replace(/-+$/, "");
  return out;
}

export const projectCodePattern = /^[a-z0-9](?:[a-z0-9-]{0,62}[a-z0-9])?$/;

export function validateProjectCode(code: string): string | null {
  const normalized = normalizeProjectCode(code);
  if (!normalized) return "项目编码不能为空";
  if (!projectCodePattern.test(normalized)) {
    return "仅允许小写字母、数字与连字符，且不能以连字符开头或结尾";
  }
  return null;
}
