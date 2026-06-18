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
  task: '任务',
  chat: '对话',
  spec: '规格',
  implement: '实现',
  verify: '验证',
  build: '构建',
  pipeline: '流水线',
}

/** Harness 各阶段在 Runs 页的说明文案 */
export const harnessKindHints: Record<string, string> = {
  spec: '结合工作区源代码编写可验收的需求文档（.matrix/SPEC-*.md）',
  implement: '根据需求文档完成编码实现与必要自测',
  verify: '对照需求验收当前实现并生成评测报告',
  build: '执行构建命令并报告结果',
}

export const settingsTabs = (projectId: string) => [
  { key: 'general', label: '常规', to: `/projects/${projectId}/-/settings/general` },
  { key: 'repositories', label: '仓库', to: `/projects/${projectId}/-/settings/repositories` },
  { key: 'members', label: '成员', to: `/projects/${projectId}/-/settings/members` },
  { key: 'integrations', label: '集成', to: `/projects/${projectId}/-/settings/integrations` },
]
