import type { CSSProperties } from "react";
import XMarkdown from "@ant-design/x-markdown";
import "@ant-design/x-markdown/themes/light.css";
import { theme } from "antd";
import "./markdown-view.scss";

type MarkdownViewProps = {
  content: string;
  className?: string;
  /** 对话气泡内渲染时使用更紧凑的间距 */
  variant?: "default" | "chat";
  streaming?: boolean;
};

export default function MarkdownView({
  content,
  className,
  variant = "default",
  streaming = false,
}: MarkdownViewProps) {
  const { token } = theme.useToken();

  const themeVars = {
    "--primary-color": token.colorPrimary,
    "--primary-color-hover": token.colorPrimary,
    "--heading-color": token.colorText,
    "--text-color": token.colorText,
    "--border-color": token.colorBorderSecondary,
    "--line-color": token.colorBorderSecondary,
    "--light-bg": token.colorFillAlter,
    "--table-head-bg": token.colorFillAlter,
    "--table-body-bg": token.colorBgContainer,
    "--cite-bg": token.colorFillTertiary,
    "--cite-hover-bg": token.colorFillSecondary,
  } as CSSProperties;

  const mergedClassName = [
    "x-markdown-light",
    "matrix-markdown",
    variant === "chat" ? "matrix-markdown--chat" : "",
    className,
  ]
    .filter(Boolean)
    .join(" ");

  return (
    <XMarkdown
      content={content}
      className={mergedClassName}
      style={themeVars}
      streaming={
        streaming
          ? { hasNextChunk: true, enableAnimation: false }
          : { hasNextChunk: false }
      }
    />
  );
}
