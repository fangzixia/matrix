import type { XProviderProps } from "@ant-design/x";
import { MATRIX_COLORS, MATRIX_SHADOW } from "./colors";

/** @ant-design/x 全局组件样式（与 antd-theme 视觉一致） */
export const matrixXProviderConfig: Pick<
  XProviderProps,
  "card" | "conversations" | "prompts" | "sender" | "welcome"
> = {
  card: { variant: "outlined" },
  conversations: {
    styles: {
      root: {
        background: MATRIX_COLORS.fillAlter,
      },
      creation: {
        borderRadius: 8,
        fontWeight: 500,
      },
      item: {
        borderRadius: 8,
        marginBlock: 2,
      },
    },
  },
  prompts: {
    styles: {
      list: {
        gap: 12,
        maxWidth: 720,
        width: "100%",
      },
      item: {
        flex: "1 1 200px",
        border: `1px solid ${MATRIX_COLORS.borderSecondary}`,
        borderRadius: 10,
        background: MATRIX_COLORS.bgContainer,
        boxShadow: MATRIX_SHADOW.card,
        padding: "14px 16px",
        cursor: "pointer",
        transition: "border-color 0.2s, box-shadow 0.2s",
      },
      itemContent: {
        gap: 4,
      },
      title: {
        fontWeight: 600,
        color: MATRIX_COLORS.text,
      },
    },
  },
  sender: {
    styles: {
      root: {
        border: `1px solid ${MATRIX_COLORS.borderSecondary}`,
        borderRadius: 12,
        boxShadow: MATRIX_SHADOW.card,
        background: MATRIX_COLORS.bgContainer,
      },
      input: {
        minHeight: 44,
      },
    },
  },
  welcome: {
    styles: {
      root: {
        maxWidth: 480,
      },
    },
  },
};
