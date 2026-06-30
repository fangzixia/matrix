import { useEffect, useState } from "react";
import { Link, useParams } from "react-router-dom";
import {
  Alert,
  Button,
  Card,
  Descriptions,
  Drawer,
  Empty,
  Flex,
  List,
  Space,
  Spin,
  Tag,
  Typography,
} from "antd";
import { GlobalOutlined, LockOutlined } from "@ant-design/icons";
import { useProjectStore } from "@/stores/project";
import * as projectsApi from "@/api/projects";
import MarkdownView from "@/components/docs/MarkdownView";

function planStatusLabel(status?: string) {
  if (status === "approved") return "已批准";
  return "待确认";
}

function planStatusColor(status?: string) {
  if (status === "approved") return "success";
  return "warning";
}

type DocItem = projectsApi.PlanItem | projectsApi.EvaluationItem;

export default function OverviewPage() {
  const { id: projectId = "" } = useParams();
  const current = useProjectStore((s) => s.current);
  const [plans, setPlans] = useState<projectsApi.PlanItem[]>([]);
  const [evaluations, setEvaluations] = useState<projectsApi.EvaluationItem[]>(
    [],
  );
  const [loading, setLoading] = useState(true);
  const [plansError, setPlansError] = useState("");
  const [evaluationsError, setEvaluationsError] = useState("");
  const [preview, setPreview] = useState<{
    title: string;
    content: string;
    path: string;
  } | null>(null);
  useEffect(() => {
    if (!projectId) return;
    setLoading(true);
    setPlansError("");
    setEvaluationsError("");
    Promise.allSettled([
      projectsApi.listPlans(projectId),
      projectsApi.listEvaluations(projectId),
    ]).then(([planRes, ev]) => {
      if (planRes.status === "fulfilled") {
        setPlans(planRes.value.plans ?? []);
      } else {
        setPlansError(
          planRes.reason instanceof Error
            ? planRes.reason.message
            : "计划文档加载失败",
        );
      }
      if (ev.status === "fulfilled") {
        setEvaluations(ev.value.evaluations ?? []);
      } else {
        setEvaluationsError(
          ev.reason instanceof Error ? ev.reason.message : "评测报告加载失败",
        );
      }
      setLoading(false);
    });
  }, [projectId]);
  const visibility = current?.visibility || "private";
  function openDoc(item: DocItem) {
    setPreview({
      title: item.title || item.path,
      content: item.content || "（无内容）",
      path: item.path,
    });
  }
  if (loading && !current) {
    return <Spin style={{ display: "block", margin: "48px auto" }} />;
  }
  return (
    <>
      {current && (
        <Flex vertical gap={8} style={{ marginBottom: 20 }}>
          <Typography.Title level={2} style={{ margin: 0 }}>
            {current.name}
          </Typography.Title>
          <Space wrap>
            <Tag
              icon={
                visibility === "private" ? <LockOutlined /> : <GlobalOutlined />
              }
              title={projectsApi.visibilityTitles[visibility]}
            >
              {projectsApi.visibilityLabels[visibility]}
            </Tag>
            {current.current_user_role && (
              <Tag color="blue">
                {projectsApi.roleLabels[current.current_user_role]}
              </Tag>
            )}
          </Space>
        </Flex>
      )}
      <Space direction="vertical" size="middle" style={{ width: "100%" }}>
        <Card title="项目信息">
          <Descriptions column={1} size="small" colon>
            <Descriptions.Item label="项目编码">
              <code>{current?.path || "—"}</code>
            </Descriptions.Item>
            <Descriptions.Item label="Git 仓库">
              {current?.git_url || "—"}
            </Descriptions.Item>
            <Descriptions.Item label="默认分支">
              <code>{current?.git_branch || "main"}</code>
            </Descriptions.Item>
          </Descriptions>
        </Card>
        <Card title="计划文档">
          {plansError && (
            <Alert
              type="warning"
              showIcon
              message={plansError}
              style={{ marginBottom: 12 }}
            />
          )}
          {plans.length ? (
            <List
              dataSource={plans}
              renderItem={(item) => (
                <List.Item
                  actions={[
                    <Button
                      key="preview"
                      type="link"
                      size="small"
                      onClick={() => openDoc(item)}
                    >
                      预览
                    </Button>,
                  ]}
                >
                  <List.Item.Meta
                    title={
                      <Space size={8}>
                        <span>{item.title || item.path}</span>
                        <Tag color={planStatusColor(item.status)}>
                          {planStatusLabel(item.status)}
                        </Tag>
                      </Space>
                    }
                    description={
                      <Typography.Text
                        type="secondary"
                        style={{ fontSize: 12 }}
                      >
                        {item.path}
                      </Typography.Text>
                    }
                  />
                </List.Item>
              )}
            />
          ) : (
            <Empty
              image={Empty.PRESENTED_IMAGE_SIMPLE}
              description="暂无计划文档"
            >
              <Link to={`/projects/${projectId}/plan`}>前往编写计划</Link>
            </Empty>
          )}
        </Card>
        <Card title="评测报告">
          {evaluationsError && (
            <Alert
              type="warning"
              showIcon
              message={evaluationsError}
              style={{ marginBottom: 12 }}
            />
          )}
          {evaluations.length ? (
            <List
              dataSource={evaluations}
              renderItem={(item) => (
                <List.Item
                  onClick={() => openDoc(item)}
                  style={{ cursor: "pointer" }}
                >
                  <List.Item.Meta
                    title={item.title || item.path}
                    description={
                      <Typography.Text
                        type="secondary"
                        style={{ fontSize: 12 }}
                      >
                        {item.path}
                        {"plan_path" in item && item.plan_path
                          ? ` · 关联计划: ${item.plan_path}`
                          : ""}
                      </Typography.Text>
                    }
                  />
                </List.Item>
              )}
            />
          ) : (
            <Empty
              image={Empty.PRESENTED_IMAGE_SIMPLE}
              description="暂无评测报告"
            />
          )}
        </Card>
      </Space>
      <Drawer
        title={preview?.title}
        open={!!preview}
        onClose={() => setPreview(null)}
        width={720}
      >
        {preview && (
          <>
            <Typography.Text
              type="secondary"
              style={{ display: "block", marginBottom: 12, fontSize: 12 }}
            >
              {preview.path}
            </Typography.Text>
            <MarkdownView content={preview.content} />
          </>
        )}
      </Drawer>
    </>
  );
}
