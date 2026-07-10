import { useEffect, useMemo, useState } from "react";
import { Alert, Button, Drawer, List, Space, Typography } from "antd";
import type { PlanItem } from "@/api/projects";
import * as projectsApi from "@/api/projects";
import { confirmDisplayGroups, parsePlanSections } from "@/utils/planParse";

type Props = {
  projectId: string;
  plan: PlanItem | null;
  open: boolean;
  onClose: () => void;
  onApproved: () => void;
};

export default function PlanConfirmDrawer({
  projectId,
  plan,
  open,
  onClose,
  onApproved,
}: Props) {
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);
  const groups = useMemo(
    () =>
      plan?.content ? confirmDisplayGroups(parsePlanSections(plan.content)) : [],
    [plan?.content],
  );
  useEffect(() => {
    if (!open || !plan) return;
    setError("");
  }, [open, plan]);
  async function submit() {
    if (!plan) return;
    setError("");
    setLoading(true);
    try {
      await projectsApi.approvePlan(projectId, {
        path: plan.path,
        resolutions: {},
      });
      onApproved();
      onClose();
    } catch (e) {
      setError(e instanceof Error ? e.message : "确认失败");
    } finally {
      setLoading(false);
    }
  }
  return (
    <Drawer
      title={`确认计划：${plan?.title || plan?.path || ""}`}
      open={open}
      onClose={onClose}
      width={640}
      extra={
        <Space>
          <Button onClick={onClose}>取消</Button>
          <Button type="primary" loading={loading} onClick={submit}>
            批准计划
          </Button>
        </Space>
      }
    >
      {error && (
        <Alert type="error" message={error} style={{ marginBottom: 16 }} />
      )}
      {plan?.status === "approved" && (
        <Alert
          type="success"
          message="该计划已批准"
          showIcon
          style={{ marginBottom: 16 }}
        />
      )}
      <Typography.Paragraph type="secondary" style={{ marginBottom: 16 }}>
        批准表示同意系统用户验收场景；附录中的技术验收标准供开发与自动验收使用。
      </Typography.Paragraph>
      {groups.length === 0 ? (
        <Typography.Paragraph type="secondary">
          计划正文结构完整，无额外待确认项，可直接批准。
        </Typography.Paragraph>
      ) : (
        <Space direction="vertical" size="middle" style={{ width: "100%" }}>
          <Typography.Paragraph type="secondary" style={{ marginBottom: 0 }}>
            批准前请知悉以下内容：
          </Typography.Paragraph>
          {groups.map((group) => (
            <div key={group.section}>
              <Typography.Text strong>{group.section}</Typography.Text>
              <List
                size="small"
                dataSource={group.items}
                renderItem={(text) => <List.Item>{text}</List.Item>}
                style={{ marginTop: 4 }}
              />
            </div>
          ))}
        </Space>
      )}
    </Drawer>
  );
}
