/** 界面默认中文文案 */

export const runStatusLabels: Record<string, string> = {
  succeeded: "成功",
  failed: "失败",
  running: "运行中",
  queued: "排队中",
  cancelled: "已取消",
  pending: "等待中",
};

import type { StageKind } from "@/types/runKind";

export const runKindLabels: Record<StageKind, string> = {
  plan: "计划",
  implement: "实现",
  verify: "验证",
  build: "构建",
};

export const stageTitles: Record<StageKind, string> = {
  plan: "编写计划",
  implement: "编码实现",
  verify: "验证评测",
  build: "执行构建",
};

/** Harness 各阶段说明文案 */
export const harnessKindHints: Record<StageKind, string> = {
  plan: "从系统用户角度描述需求如何被满足，生成易读的计划文档（含验收场景与附录）",
  implement: "根据计划文档完成编码实现与必要自测",
  verify: "以系统用户身份实操验收，生成需求满足度评测报告（docs/evaluations/EVAL-*.md）",
  build: "按计划完成实现与验收闭环",
};

export const settingsTabs = (projectId: string) => [
  {
    key: "general",
    label: "常规",
    to: `/projects/${projectId}/-/settings/general`,
  },
  {
    key: "repositories",
    label: "仓库",
    to: `/projects/${projectId}/-/settings/repositories`,
  },
  {
    key: "members",
    label: "成员",
    to: `/projects/${projectId}/-/settings/members`,
  },
];
