import { useState } from "react";
import { useNavigate } from "react-router-dom";
import {
  Alert,
  Button,
  Card,
  Flex,
  Form,
  Input,
  Radio,
  Space,
  Typography,
} from "antd";
import * as projectsApi from "@/api/projects";
import type { ProjectVisibility } from "@/api/projects";
import {
  inferProjectCodeFromGitUrl,
  validateProjectCode,
} from "@/utils/projectCode";

const visibilityOptions: {
  value: ProjectVisibility;
  label: string;
  hint: string;
}[] = [
  { value: "private", label: "私有", hint: "仅被明确授权的用户可访问。" },
  { value: "internal", label: "内部", hint: "所有已登录用户可访问。" },
  { value: "public", label: "公开", hint: "所有已登录用户可访问。" },
];

export default function ProjectNewPage() {
  const navigate = useNavigate();
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);
  const [form] = Form.useForm();
  const [pathTouched, setPathTouched] = useState(false);
  async function onFinish(values: {
    name: string;
    path: string;
    git_url?: string;
    git_branch?: string;
    visibility: ProjectVisibility;
  }) {
    setError("");
    setLoading(true);
    try {
      const p = await projectsApi.createProject({
        name: values.name,
        path: values.path,
        git_url: values.git_url || undefined,
        git_branch: values.git_branch || undefined,
        visibility: values.visibility,
      });
      navigate(`/projects/${p.id}`);
    } catch (e) {
      setError(e instanceof Error ? e.message : "创建项目失败");
    } finally {
      setLoading(false);
    }
  }
  function suggestCodeFromGit() {
    if (pathTouched) return;
    const gitUrl = form.getFieldValue("git_url") as string | undefined;
    const suggested = inferProjectCodeFromGitUrl(gitUrl || "");
    if (suggested) form.setFieldValue("path", suggested);
  }
  return (
    <>
      <Typography.Title level={2}>创建新项目</Typography.Title>
      {error && (
        <Alert type="error" message={error} style={{ marginBottom: 16 }} />
      )}
      <Card style={{ maxWidth: 560 }}>
        <Form
          form={form}
          layout="vertical"
          initialValues={{ git_branch: "main", visibility: "private" }}
          onFinish={onFinish}
        >
          <Form.Item
            label="项目名称"
            name="name"
            rules={[{ required: true, message: "请输入项目名称" }]}
          >
            <Input placeholder="my-awesome-project" />
          </Form.Item>
          <Form.Item
            label="项目编码"
            name="path"
            extra="必填。填写 Git 地址后失焦可自动建议编码；保存时将自动规范化为小写与合法字符"
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
            <Input
              placeholder="my-project"
              onChange={() => setPathTouched(true)}
            />
          </Form.Item>
          <Form.Item label="Git 地址（可选）" name="git_url">
            <Input
              placeholder="https://gitlab.example.com/group/project.git"
              onBlur={suggestCodeFromGit}
            />
          </Form.Item>
          <Form.Item label="默认分支" name="git_branch">
            <Input placeholder="main" />
          </Form.Item>
          <Form.Item label="可见性" name="visibility">
            <Radio.Group>
              <Space direction="vertical">
                {visibilityOptions.map((opt) => (
                  <Radio key={opt.value} value={opt.value}>
                    <Typography.Text strong>{opt.label}</Typography.Text>
                    <Typography.Text type="secondary" style={{ marginLeft: 8 }}>
                      {opt.hint}
                    </Typography.Text>
                  </Radio>
                ))}
              </Space>
            </Radio.Group>
          </Form.Item>
          <Flex justify="space-between">
            <Button onClick={() => navigate("/projects")}>取消</Button>
            <Button type="primary" htmlType="submit" loading={loading}>
              创建项目
            </Button>
          </Flex>
        </Form>
      </Card>
    </>
  );
}
