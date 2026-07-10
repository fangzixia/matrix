/** 解析计划 Markdown 章节，供确认 UI 使用。 */

export type PlanSectionItems = {
  summary: string[];
  scopeIncluded: string[];
  scopeExcluded: string[];
  userAcceptance: string[];
  risks: string[];
  clarifications: string[];
  conflicts: string[];
};

export type PlanConfirmGroup = {
  section: string;
  items: string[];
};

export function parsePlanSections(content: string): PlanSectionItems {
  const scopeSection = extractSectionAny(content, ["范围"]);
  const clarifications = [
    ...extractBullets(extractSectionAny(content, ["待确认"])),
    ...extractBullets(
      extractSectionAny(content, ["待优化 / 待澄清"]),
    ),
  ];
  const userAcceptance = extractBullets(
    extractSectionAny(content, ["用户验收（产品语言）"]),
  );
  return {
    summary: extractBullets(extractSectionAny(content, ["摘要"])),
    scopeIncluded: extractSubsectionBullets(scopeSection, "本次包含"),
    scopeExcluded: extractSubsectionBullets(scopeSection, "本次不包含"),
    userAcceptance:
      userAcceptance.length > 0
        ? userAcceptance
        : extractLegacyAcceptance(content),
    risks: extractBullets(extractSectionAny(content, ["风险"])),
    clarifications: dedupe(clarifications),
    conflicts: extractBullets(extractSectionAny(content, ["冲突与依赖"])),
  };
}

/** 按确认抽屉展示顺序分组；范围各取前 3 条，用户验收最多 5 条。 */
export function confirmDisplayGroups(
  items: PlanSectionItems,
): PlanConfirmGroup[] {
  const groups: PlanConfirmGroup[] = [];

  const included = items.scopeIncluded.slice(0, 3);
  const excluded = items.scopeExcluded.slice(0, 3);
  const scopeItems = [
    ...included.map((t) => `包含：${t}`),
    ...excluded.map((t) => `不包含：${t}`),
  ];
  if (scopeItems.length > 0) {
    groups.push({ section: "范围摘要", items: scopeItems });
  }

  if (items.userAcceptance.length > 0) {
    groups.push({
      section: "用户验收",
      items: items.userAcceptance.slice(0, 5),
    });
  }

  if (items.risks.length > 0) {
    groups.push({ section: "风险", items: items.risks });
  }

  if (items.clarifications.length > 0) {
    groups.push({ section: "待确认", items: items.clarifications });
  }

  if (items.conflicts.length > 0) {
    groups.push({ section: "冲突与依赖", items: items.conflicts });
  }

  return groups;
}

function extractSectionAny(content: string, headings: string[]): string {
  for (const heading of headings) {
    const section = extractSection(content, heading);
    if (section.trim()) return section;
  }
  return "";
}

function extractSection(content: string, heading: string): string {
  const lines = content.split("\n");
  const buf: string[] = [];
  let inSection = false;
  for (const line of lines) {
    const trim = line.trim();
    if (trim.startsWith("## ")) {
      const title = trim.slice(3).trim();
      if (title === heading) {
        inSection = true;
        continue;
      }
      if (inSection) break;
    }
    if (inSection) buf.push(line);
  }
  return buf.join("\n");
}

function extractSubsectionBullets(section: string, subheading: string): string[] {
  const lines = section.split("\n");
  const buf: string[] = [];
  let inSub = false;
  for (const line of lines) {
    const trim = line.trim();
    if (trim.startsWith("### ")) {
      const title = trim.slice(4).trim();
      if (title === subheading) {
        inSub = true;
        continue;
      }
      if (inSub) break;
    }
    if (inSub) buf.push(line);
  }
  return extractBullets(buf.join("\n"));
}

function extractBullets(section: string): string[] {
  const out: string[] = [];
  for (const line of section.split("\n")) {
    const trim = line.trim();
    if (trim.startsWith("- ")) {
      const item = trim.slice(2).trim();
      if (item && !isNoneMarker(item)) out.push(item);
    }
  }
  return out;
}

/** 旧版计划「验收标准」章节中的 AC 行，作用户验收回退。 */
function extractLegacyAcceptance(content: string): string[] {
  const section = extractSectionAny(content, ["验收标准"]);
  const out: string[] = [];
  for (const line of section.split("\n")) {
    const trim = line.trim();
    if (trim.startsWith("- AC-")) {
      out.push(trim.slice(2).trim());
    }
  }
  return out;
}

function isNoneMarker(s: string): boolean {
  const v = s.toLowerCase().trim();
  return v === "无" || v === "暂无" || v === "none" || v === "n/a" || v === "-";
}

function dedupe(items: string[]): string[] {
  const seen = new Set<string>();
  const out: string[] = [];
  for (const item of items) {
    if (seen.has(item)) continue;
    seen.add(item);
    out.push(item);
  }
  return out;
}
