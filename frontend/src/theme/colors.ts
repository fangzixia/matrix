/** Matrix 浅色企业风色板（antd-theme 与 X 组件共用） */
export const MATRIX_COLORS = {
  primary: "#fc6d26",
  text: "#1f2937",
  textSecondary: "#6b7280",
  bgLayout: "#eef1f5",
  bgContainer: "#ffffff",
  fillAlter: "#f9fafb",
  fillSecondary: "#f3f4f6",
  border: "#d1d5db",
  borderSecondary: "#e5e7eb",
  primaryBg: "#fff5f0",
  primaryBorder: "#fdd9c8",
  /** Logo 印记色（GitLab 风格） */
  brandMark: "#e24329",
  brandAccent: "#fca326",
} as const;

export const MATRIX_SHADOW = {
  card: "0 1px 3px rgba(15, 23, 42, 0.07), 0 1px 2px rgba(15, 23, 42, 0.04)",
  elevated:
    "0 4px 14px rgba(15, 23, 42, 0.08), 0 2px 6px rgba(15, 23, 42, 0.04)",
  header: "0 1px 0 rgba(15, 23, 42, 0.05), 0 1px 3px rgba(15, 23, 42, 0.04)",
} as const;
