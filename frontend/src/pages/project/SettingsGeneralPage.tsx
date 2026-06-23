import { useEffect, useState } from "react";
import { useNavigate, useParams } from "react-router-dom";
import {
  Alert,
  Button,
  Card,
  Form,
  Input,
  Modal,
  Select,
  Tabs,
  Typography,
} from "antd";
import { useProjectStore } from "@/stores/project";
import { useProjectPermissions } from "@/hooks/useProjectPermissions";
import { useSettingsTabNavigate } from "@/hooks/useSettingsTabNavigate";
import * as projectsApi from "@/api/projects";
import * as groupsApi from "@/api/groups";
import type { ProjectVisibility } from "@/api/projects";
import type { Group } from "@/api/groups";
import { settingsTabs } from "@/locales/zh-CN";
import { validateProjectCode } from "@/utils/projectCode";

export default function SettingsGeneralPage() {
  const { id = "" } = useParams();
  const navigate = useNavigate();
  const projectStore = useProjectStore();
  const perms = useProjectPermissions(projectStore.current);
  const onSettingsTabChange = useSettingsTabNavigate(id);
  const [error, setError] = useState("");
  const [message, setMessage] = useState("");
  const [deleteOpen, setDeleteOpen] = useState(false);
  const [groups, setGroups] = useState<Group[]>([]);
  const [form] = Form.useForm();
  useEffect(() => {
    groupsApi.listGroups().then((res) => setGroups(res.groups));
  }, []);
  useEffect(() => {
    const p = projectStore.current;
    if (!p) return;
    form.setFieldsValue({
      name: p.name,
      path: p.path || "",
      git_url: p.git_url,
      git_branch: p.git_branch,
      visibility: p.visibility || "private",
      group_id: p.group_id || "",
    });
  }, [projectStore.current, form]);
  async function onFinish(values: {
    name: string;
    path: string;
    git_url: string;
    git_branch: string;
    visibility: ProjectVisibility;
    group_id: string;
  }) {
    setError("");
    setMessage("");
    try {
      await projectsApi.updateProject(id, {
        ...values,
        group_id: values.group_id || null,
      });
      await projectStore.fetchProject(id);
      setMessage("设置已保存。");
    } catch (e) {
      setError(e instanceof Error ? e.message : "保存失败");
    }
  }
  async function confirmDelete() {
    await projectsApi.deleteProject(id);
    setDeleteOpen(false);
    navigate("/projects");
  }
  if (!perms.canManageSettings) {
    return <Alert type="error" message="您没有权限访问项目设置。" />;
  }
  return (
    <div>
      <Tabs
        activeKey="general"
        onChange={onSettingsTabChange}
        items={settingsTabs(id).map((tab) => ({
          key: tab.key,
          label: tab.label,
        }))}
        style={{ marginBottom: 16 }}
      />
      <Typography.Title level={2}>常规</Typography.Title>
      {error && (
        <Alert type="error" message={error} style={{ marginBottom: 16 }} />
      )}
      {message && (
        <Alert type="success" message={message} style={{ marginBottom: 16 }} />
      )}
      <Card style={{ maxWidth: 560, marginBottom: 24 }}>
        <Form form={form} layout="vertical" onFinish={onFinish}>
          <Form.Item label="项目名称" name="name" rules={[{ required: true }]}>
            <Input />
          </Form.Item>
          <Form.Item
            label="项目编码"
            name="path"
            rules={[
              { required: true, message: "请输入项目编码" },
              {
                validator: async (_, value) => {
                  const msg = validateProjectCode(value || "");
                  if (msg) throw new Error(msg);
                },
              },
            ]}
          >
            <Input placeholder="my-project" />
          </Form.Item>
          <Form.Item label="所属组" name="group_id">
            <Select
              allowClear
              placeholder="— 无 —"
              options={groups.map((g) => ({ value: g.id, label: g.name }))}
            />
          </Form.Item>
          <Form.Item label="Git 地址" name="git_url">
            <Input />
          </Form.Item>
          <Form.Item label="默认分支" name="git_branch">
            <Input />
          </Form.Item>
          <Form.Item label="可见性" name="visibility">
            <Select
              options={[
                { value: "private", label: "私有" },
                { value: "internal", label: "内部" },
                { value: "public", label: "公开" },
              ]}
            />
          </Form.Item>
          <Button type="primary" htmlType="submit">
            保存更改
          </Button>
        </Form>
      </Card>
      {perms.canDeleteProject && (
        <Card
          style={{ maxWidth: 560 }}
          styles={{ body: { borderColor: "#dd2b0e" } }}
        >
          <Typography.Title level={4} type="danger">
            危险操作
          </Typography.Title>
          <Typography.Text type="secondary">
            删除项目后无法恢复，所有运行记录与成员关系将被清除。
          </Typography.Text>
          <br />
          <Button
            danger
            onClick={() => setDeleteOpen(true)}
            style={{ marginTop: 12 }}
          >
            删除项目
          </Button>
        </Card>
      )}
      <Modal
        open={deleteOpen}
        title="删除项目"
        onCancel={() => setDeleteOpen(false)}
        onOk={confirmDelete}
        okText="删除"
        okButtonProps={{ danger: true }}
      >
        确定删除项目 <strong>{form.getFieldValue("name")}</strong>
        ？此操作不可撤销。
      </Modal>
    </div>
  );
}
