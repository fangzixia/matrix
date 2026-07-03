import { useEffect, useMemo, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import {
  Avatar,
  Button,
  Card,
  Empty,
  Flex,
  Input,
  List,
  Tabs,
  Tag,
  Typography,
  theme,
} from "antd";
import { GlobalOutlined, LockOutlined, PlusOutlined } from "@ant-design/icons";
import { useProjectStore } from "@/stores/project";
import * as projectsApi from "@/api/projects";
import { avatarInitials } from "@/utils/avatar";
import { listCardProps } from "@/theme/surface";

export default function ProjectsPage() {
  const { token } = theme.useToken();
  const cardProps = listCardProps(token);
  const navigate = useNavigate();
  const projects = useProjectStore((s) => s.projects);
  const fetchProjects = useProjectStore((s) => s.fetchProjects);
  const [scope, setScope] = useState<"yours" | "explore">("yours");
  const [filter, setFilter] = useState("");
  const filtered = useMemo(() => {
    const q = filter.trim().toLowerCase();
    if (!q) return projects;
    return projects.filter((p) => p.name.toLowerCase().includes(q));
  }, [filter, projects]);
  useEffect(() => {
    fetchProjects(scope);
  }, [scope, fetchProjects]);
  return (
    <>
      <Flex justify="space-between" align="center" style={{ marginBottom: 16 }}>
        <Typography.Title level={2} style={{ margin: 0 }}>
          项目
        </Typography.Title>
        <Button
          type="primary"
          icon={<PlusOutlined />}
          onClick={() => navigate("/projects/new")}
        >
          新建项目
        </Button>
      </Flex>
      <Tabs
        activeKey={scope}
        onChange={(key) => setScope(key as "yours" | "explore")}
        items={[
          { key: "yours", label: "我的项目" },
          { key: "explore", label: "探索项目" },
        ]}
        style={{ marginBottom: 16 }}
      />
      <Input
        value={filter}
        onChange={(e) => setFilter(e.target.value)}
        placeholder="按名称筛选…"
        allowClear
        style={{ maxWidth: 360, marginBottom: 16 }}
      />
      {filtered.length ? (
        <List
          dataSource={filtered}
          renderItem={(p) => {
            const visibility = p.visibility || "private";
            return (
              <List.Item style={{ padding: 0, border: "none" }}>
                <Card
                  {...cardProps}
                  style={{ ...cardProps.style, width: "100%" }}
                  styles={{ body: { padding: "16px 20px" } }}
                >
                  <Flex gap={16} align="flex-start">
                    <Link to={`/projects/${p.id}`}>
                      <Avatar
                        size={48}
                        shape="square"
                        style={{ backgroundColor: token.colorPrimary }}
                      >
                        {avatarInitials(p.name)}
                      </Avatar>
                    </Link>
                    <Flex vertical flex={1} gap={4}>
                      <Flex align="center" gap={8} wrap="wrap">
                        <Typography.Title level={5} style={{ margin: 0 }}>
                          <Link to={`/projects/${p.id}`}>{p.name}</Link>
                        </Typography.Title>
                        <Tag
                          icon={
                            visibility === "private" ? (
                              <LockOutlined />
                            ) : (
                              <GlobalOutlined />
                            )
                          }
                          title={projectsApi.visibilityTitles[visibility]}
                        >
                          {projectsApi.visibilityLabels[visibility]}
                        </Tag>
                        {p.current_user_role && (
                          <Tag color="blue">
                            {projectsApi.roleLabels[p.current_user_role]}
                          </Tag>
                        )}
                      </Flex>
                      {p.git_url && (
                        <Typography.Text type="secondary">
                          {p.git_url}
                        </Typography.Text>
                      )}
                      <Typography.Text
                        type="secondary"
                        style={{ fontSize: 12 }}
                      >
                        更新于 {projectsApi.formatRelativeTime(p.updated_at)}
                      </Typography.Text>
                    </Flex>
                  </Flex>
                </Card>
              </List.Item>
            );
          }}
        />
      ) : (
        <Empty
          description={
            scope === "explore"
              ? "暂无可探索的内部或公开项目"
              : "还没有项目 — 创建第一个项目吧。"
          }
        />
      )}
    </>
  );
}
