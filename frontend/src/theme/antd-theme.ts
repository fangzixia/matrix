import type { ThemeConfig } from "antd";
import { MATRIX_COLORS, MATRIX_SHADOW } from "./colors";

/** Matrix 浅色企业风设计令牌（Notion / GitLab 气质） */
export const matrixTheme: ThemeConfig = {
  token: {
    colorPrimary: MATRIX_COLORS.primary,
    colorSuccess: "#108548",
    colorError: "#dd2b0e",
    colorWarning: "#ab6100",
    colorInfo: "#1f75cb",
    colorLink: "#1f75cb",
    colorText: MATRIX_COLORS.text,
    colorTextSecondary: MATRIX_COLORS.textSecondary,
    colorTextTertiary: "#9ca3af",
    colorBgLayout: MATRIX_COLORS.bgLayout,
    colorBgContainer: MATRIX_COLORS.bgContainer,
    colorBgElevated: MATRIX_COLORS.bgContainer,
    colorBorder: MATRIX_COLORS.border,
    colorBorderSecondary: MATRIX_COLORS.borderSecondary,
    colorFillAlter: MATRIX_COLORS.fillAlter,
    colorFillSecondary: MATRIX_COLORS.fillSecondary,
    colorFillTertiary: "#e5e7eb",
    colorPrimaryBg: MATRIX_COLORS.primaryBg,
    colorPrimaryBgHover: "#ffe8de",
    colorPrimaryBorder: MATRIX_COLORS.primaryBorder,
    colorPrimaryBorderHover: "#fcb896",
    borderRadius: 8,
    borderRadiusLG: 10,
    borderRadiusSM: 6,
    lineWidth: 1,
    fontSize: 14,
    fontSizeHeading1: 30,
    fontSizeHeading2: 24,
    fontSizeHeading3: 20,
    fontSizeHeading4: 16,
    fontSizeHeading5: 14,
    lineHeight: 1.5715,
    controlHeight: 36,
    fontFamily:
      "'Inter', -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif",
    fontFamilyCode:
      "'JetBrains Mono', ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace",
    boxShadow: MATRIX_SHADOW.header,
    boxShadowSecondary: MATRIX_SHADOW.elevated,
    boxShadowTertiary: MATRIX_SHADOW.card,
    motionDurationMid: "0.2s",
    motionDurationSlow: "0.3s",
  },
  components: {
    Layout: {
      headerHeight: 48,
      headerBg: MATRIX_COLORS.bgContainer,
      siderBg: MATRIX_COLORS.fillAlter,
      bodyBg: MATRIX_COLORS.bgLayout,
      footerBg: "transparent",
    },
    Menu: {
      itemSelectedColor: MATRIX_COLORS.primary,
      itemSelectedBg: MATRIX_COLORS.primaryBg,
      itemHoverBg: MATRIX_COLORS.fillSecondary,
      itemHoverColor: MATRIX_COLORS.text,
      activeBarWidth: 3,
      iconSize: 16,
      itemHeight: 40,
    },
    Card: {
      borderRadiusLG: 10,
      paddingLG: 20,
      headerBg: MATRIX_COLORS.fillAlter,
      boxShadowTertiary: MATRIX_SHADOW.card,
    },
    Button: {
      borderRadius: 8,
      controlHeight: 36,
      paddingContentHorizontal: 16,
      primaryShadow: "0 1px 2px rgba(252, 109, 38, 0.2)",
    },
    Input: {
      borderRadius: 8,
      controlHeight: 36,
      paddingInline: 12,
      activeBorderColor: MATRIX_COLORS.primary,
      hoverBorderColor: MATRIX_COLORS.border,
    },
    Select: {
      borderRadius: 8,
      controlHeight: 36,
    },
    Table: {
      borderRadius: 10,
      headerBg: MATRIX_COLORS.fillAlter,
      headerColor: MATRIX_COLORS.textSecondary,
      rowHoverBg: "#f8fafc",
      borderColor: MATRIX_COLORS.borderSecondary,
      headerBorderRadius: 10,
    },
    Tabs: {
      inkBarColor: MATRIX_COLORS.primary,
      itemSelectedColor: MATRIX_COLORS.primary,
      itemHoverColor: MATRIX_COLORS.primary,
      titleFontSize: 14,
    },
    Tag: {
      borderRadiusSM: 6,
    },
    List: {
      itemPadding: "12px 0",
    },
    Typography: {
      titleMarginBottom: "0.5em",
      titleMarginTop: 0,
    },
    Breadcrumb: {
      fontSize: 13,
    },
    Dropdown: {
      borderRadiusLG: 10,
    },
    Modal: {
      borderRadiusLG: 12,
    },
    Drawer: {
      borderRadiusLG: 12,
    },
    Alert: {
      borderRadiusLG: 8,
    },
    Form: {
      labelFontSize: 14,
      verticalLabelPadding: "0 0 6px",
    },
    Descriptions: {
      borderRadiusLG: 10,
      labelBg: MATRIX_COLORS.fillAlter,
    },
    Badge: {
      borderRadiusSM: 6,
    },
    Splitter: {
      splitBarSize: 4,
      splitTriggerSize: 12,
    },
  },
};
