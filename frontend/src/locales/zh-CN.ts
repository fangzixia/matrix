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

export const settingsTabs = (projectId: string) => [
  { key: 'general', label: '常规', to: `/projects/${projectId}/-/settings/general` },
  { key: 'repositories', label: '仓库', to: `/projects/${projectId}/-/settings/repositories` },
  { key: 'members', label: '成员', to: `/projects/${projectId}/-/settings/members` },
  { key: 'integrations', label: '集成', to: `/projects/${projectId}/-/settings/integrations` },
]
