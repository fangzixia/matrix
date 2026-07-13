import type { CSSProperties } from "react";
import type { GlobalToken } from "antd/es/theme/interface";

/** 页面级信息卡片（带标题栏、边框与阴影） */
export function pageCardProps(token: GlobalToken) {
  return {
    variant: "outlined" as const,
    style: {
      boxShadow: token.boxShadowTertiary,
      borderColor: token.colorBorder,
    } satisfies CSSProperties,
    styles: {
      header: {
        background: token.colorFillAlter,
        borderBottom: `1px solid ${token.colorBorderSecondary}`,
        fontWeight: 600,
      },
    },
  };
}

/** 列表项卡片（无标题） */
export function listCardProps(token: GlobalToken) {
  return {
    variant: "outlined" as const,
    style: {
      boxShadow: token.boxShadowTertiary,
      borderColor: token.colorBorder,
    } satisfies CSSProperties,
  };
}
