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
  Space,
  Typography,
  theme,
} from "antd";
import { useProjectStore } from "@/stores/project";
import { useProjectPermissions } from "@/hooks/useProjectPermissions";
import * as projectsApi from "@/api/projects";
import * as groupsApi from "@/api/groups";
import type { ProjectVisibility } from "@/api/projects";
import type { Group } from "@/api/groups";
import { validateProjectCode } from "@/utils/projectCode";
import { pageCardProps } from "@/theme/surface";
import { ProjectSettingsFrame } from "@/pages/project/ProjectSettingsFrame";

export default function SettingsGeneralPage() {
  const { token } = theme.useToken();
  const cardProps = pageCardProps(token);
  const { id = "" } = useParams();
  const navigate = useNavigate();
  const projectStore = useProjectStore();
  const perms = useProjectPermissions(projectStore.current);
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
    return <Alert type="error" title="您没有权限访问项目设置。" />;
  }
  return (
    <ProjectSettingsFrame
      projectId={id}
      activeTab="general"
      sectionTitle="常规"
    >
      <Space orientation="vertical" size="middle" style={{ width: "100%" }}>
        {error && <Alert type="error" title={error} showIcon />}
        {message && <Alert type="success" title={message} showIcon />}
        <Card title="基本信息" {...cardProps} style={{ ...cardProps.style, maxWidth: 720 }}>
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
            title="危险操作"
            {...cardProps}
            style={{
              ...cardProps.style,
              maxWidth: 720,
              borderColor: token.colorErrorBorder,
            }}
            styles={{
              ...cardProps.styles,
              header: {
                ...cardProps.styles?.header,
                color: token.colorError,
              },
            }}
          >
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
      </Space>
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
    </ProjectSettingsFrame>
  );
}
