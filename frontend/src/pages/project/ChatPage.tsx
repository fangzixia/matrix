import { useParams } from "react-router-dom";
import { theme } from "antd";
import ProjectChatWorkspace from "@/components/ai/ProjectChatWorkspace";
import type { ChatPromptItem } from "@/components/ai/MatrixAiChat";

export const DEFAULT_PROMPTS: ChatPromptItem[] = [
  {
    key: "summary",
    label: "总结当前项目文档",
    description: "快速了解项目现状与关键文档",
  },
  {
    key: "plan",
    label: "帮我写实现计划",
    description: "基于需求拆解可执行的开发步骤",
  },
  {
    key: "review",
    label: "审查代码潜在问题",
    description: "从可维护性与风险角度给出建议",
  },
  {
    key: "explain",
    label: "解释项目架构",
    description: "梳理模块职责与数据流向",
  },
];

export default function ChatPage() {
  const { id: projectId = "" } = useParams();
  const { token } = theme.useToken();
  const headerHeight = token.Layout?.headerHeight ?? 48;

  return (
    <ProjectChatWorkspace
      projectId={projectId}
      prompts={DEFAULT_PROMPTS}
      welcomeTitle="开始对话"
      style={{ height: `calc(100vh - ${headerHeight}px)` }}
    />
  );
}
