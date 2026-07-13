/** Matrix 布局尺寸（与 antd Layout / 对话侧栏对齐） */
export const MATRIX_LAYOUT = {
  /** AppShell 项目侧栏宽度 */
  appSiderWidth: 220,
  /** 对话会话列表侧栏宽度 */
  chatSiderWidth: 240,
  /** 对话区活动面板最大高度（避免挤压 Bubble.List） */
  activityPanelMaxHeight: 280,
  /** 工具输出预览最大高度 */
  toolOutputMaxHeight: 320,
} as const;
