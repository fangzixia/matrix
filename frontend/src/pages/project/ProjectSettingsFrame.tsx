import type { ReactNode } from "react";
import { Tabs, Typography } from "antd";
import { useSettingsTabNavigate } from "@/hooks/useSettingsTabNavigate";
import { settingsTabs } from "@/locales/zh-CN";

interface ProjectSettingsFrameProps {
  projectId: string;
  activeTab: string;
  sectionTitle: string;
  children: ReactNode;
}

/** 项目设置页统一壳：标题 + Tabs + 分区内容（基于 antd Layout 内容区组合） */
export function ProjectSettingsFrame({
  projectId,
  activeTab,
  sectionTitle,
  children,
}: ProjectSettingsFrameProps) {
  const onTabChange = useSettingsTabNavigate(projectId);
  return (
    <>
      <Typography.Title level={2} style={{ marginTop: 0, marginBottom: 16 }}>
        设置
      </Typography.Title>
      <Tabs
        activeKey={activeTab}
        onChange={onTabChange}
        items={settingsTabs(projectId).map((tab) => ({
          key: tab.key,
          label: tab.label,
        }))}
        style={{ marginBottom: 20 }}
      />
      <Typography.Title level={4} style={{ marginTop: 0, marginBottom: 16 }}>
        {sectionTitle}
      </Typography.Title>
      {children}
    </>
  );
}
