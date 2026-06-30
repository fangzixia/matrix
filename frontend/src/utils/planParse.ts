/** 解析计划 Markdown 章节，供确认 UI 使用。 */

export type PlanSectionItems = {
  risks: string[];
  conflicts: string[];
  clarifications: string[];
};

export function parsePlanSections(content: string): PlanSectionItems {
  return {
    risks: extractBullets(extractSection(content, "风险")),
    conflicts: extractBullets(extractSection(content, "冲突与依赖")),
    clarifications: extractBullets(extractSection(content, "待优化 / 待澄清")),
  };
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

function isNoneMarker(s: string): boolean {
  const v = s.toLowerCase().trim();
  return v === "无" || v === "暂无" || v === "none" || v === "n/a" || v === "-";
}

export function confirmDisplayItems(
  items: PlanSectionItems,
): Array<{ section: string; text: string }> {
  const rows: Array<{ section: string; text: string }> = [];
  items.conflicts.forEach((text) =>
    rows.push({ section: "冲突与依赖", text }),
  );
  items.clarifications.forEach((text) =>
    rows.push({ section: "待优化 / 待澄清", text }),
  );
  return rows;
}
