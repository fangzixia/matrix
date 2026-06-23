import { useEffect, useMemo, useState } from "react";
import { Alert, Button, Drawer, Form, Input, Space, Typography } from "antd";
import type { PlanItem } from "@/api/projects";
import * as projectsApi from "@/api/projects";
import { allConfirmItems, parsePlanSections } from "@/utils/planParse";

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
  const [form] = Form.useForm();
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);
  const items = useMemo(
    () =>
      plan?.content ? allConfirmItems(parsePlanSections(plan.content)) : [],
    [plan?.content],
  );
  useEffect(() => {
    if (!open || !plan) return;
    form.resetFields();
    setError("");
  }, [open, plan, form]);
  async function submit() {
    if (!plan) return;
    setError("");
    setLoading(true);
    try {
      const values = await form.validateFields();
      const resolutions: Record<string, string> = {};
      for (const item of items) {
        resolutions[item.key] = String(values[item.key] ?? "").trim();
      }
      await projectsApi.approvePlan(projectId, {
        path: plan.path,
        resolutions,
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
      {items.length === 0 ? (
        <Typography.Paragraph type="secondary">
          计划中无待确认的风险、冲突或澄清项，可直接批准。
        </Typography.Paragraph>
      ) : (
        <Form form={form} layout="vertical">
          {items.map((item) => (
            <Form.Item
              key={item.key}
              name={item.key}
              label={`${item.section}：${item.text}`}
              rules={[{ required: true, message: "请填写确认或解决办法" }]}
            >
              <Input.TextArea rows={2} placeholder="确认意见或解决办法" />
            </Form.Item>
          ))}
        </Form>
      )}
    </Drawer>
  );
}
