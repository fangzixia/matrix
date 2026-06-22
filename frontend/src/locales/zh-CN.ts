/** 界面默认中文文案 */

export const runStatusLabels: Record<string, string> = {
  succeeded: '成功',
  failed: '失败',
  running: '运行中',
  queued: '排队中',
  cancelled: '已取消',
  pending: '等待中',
}

export const runKindLabels: Record<string, string> = {
  plan: '计划',
  implement: '实现',
  verify: '验证',
  build: '构建',
}

export const stageTitles: Record<string, string> = {
  plan: '编写计划',
  implement: '编码实现',
  verify: '验证评测',
  build: '执行构建',
}

/** Harness 各阶段说明文案 */
export const harnessKindHints: Record<string, string> = {
  plan: '结合工作区源代码编写可验收的计划文档（.matrix/PLAN-*.md）',
  implement: '根据计划文档完成编码实现与必要自测',
  verify: '对照计划验收当前实现并生成评测报告',
  build: '按计划完成实现与验收闭环',
}

export const settingsTabs = (projectId: string) => [
  { key: 'general', label: '常规', to: `/projects/${projectId}/-/settings/general` },
  { key: 'repositories', label: '仓库', to: `/projects/${projectId}/-/settings/repositories` },
  { key: 'members', label: '成员', to: `/projects/${projectId}/-/settings/members` },
  { key: 'integrations', label: '集成', to: `/projects/${projectId}/-/settings/integrations` },
]
