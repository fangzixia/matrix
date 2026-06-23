import type { ThemeConfig } from "antd";

export const matrixTheme: ThemeConfig = {
  token: {
    colorPrimary: "#fc6d26",
    colorSuccess: "#108548",
    colorError: "#dd2b0e",
    colorInfo: "#1f75cb",
    colorLink: "#1f75cb",
    colorText: "#303030",
    colorTextSecondary: "#737278",
    colorBgLayout: "#f0f0f0",
    colorBgContainer: "#ffffff",
    colorBgElevated: "#ffffff",
    colorBorder: "#dcdcde",
    colorBorderSecondary: "#ececef",
    borderRadius: 4,
    fontSize: 14,
    fontFamily:
      "-apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif",
  },
  components: {
    Layout: {
      headerHeight: 48,
      siderBg: "#ffffff",
      headerBg: "#ffffff",
      bodyBg: "#f0f0f0",
    },
    Menu: {
      itemSelectedColor: "#fc6d26",
      itemSelectedBg: "#fdf1ed",
      itemHoverBg: "#fbfafd",
    },
    Card: {
      borderRadiusLG: 4,
    },
  },
};
