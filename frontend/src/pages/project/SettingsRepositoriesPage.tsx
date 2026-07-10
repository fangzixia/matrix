import { useEffect, useState } from "react";
import { useParams } from "react-router-dom";
import {
  Alert,
  Button,
  Card,
  Checkbox,
  Form,
  Input,
  Modal,
  Space,
  Table,
  theme,
} from "antd";
import * as reposApi from "@/api/repositories";
import type { Repository } from "@/api/repositories";
import { pageCardProps } from "@/theme/surface";
import { ProjectSettingsFrame } from "@/pages/project/ProjectSettingsFrame";

export default function SettingsRepositoriesPage() {
  const { token } = theme.useToken();
  const cardProps = pageCardProps(token);
  const { id: projectId = "" } = useParams();
  const [repos, setRepos] = useState<Repository[]>([]);
  const [error, setError] = useState("");
  const [actingId, setActingId] = useState("");
  const [form] = Form.useForm();
  async function load() {
    const res = await reposApi.listRepositories(projectId);
    setRepos(res.repositories);
  }
  useEffect(() => {
    load();
  }, [projectId]);
  async function add(values: {
    name: string;
    git_url: string;
    git_branch: string;
    is_default: boolean;
  }) {
    setError("");
    try {
      await reposApi.createRepository(projectId, values);
      form.resetFields();
      form.setFieldsValue({ git_branch: "main", is_default: false });
      await load();
    } catch (e) {
      setError(e instanceof Error ? e.message : "添加失败");
    }
  }
  async function remove(r: Repository) {
    Modal.confirm({
      title: `删除仓库「${r.name}」？`,
      content: "删除后将移除该项目的仓库绑定，此操作不可撤销。",
      okText: "删除",
      okButtonProps: { danger: true },
      cancelText: "取消",
      onOk: async () => {
        setError("");
        setActingId(r.id);
        try {
          await reposApi.deleteRepository(projectId, r.id);
          await load();
        } catch (e) {
          setError(e instanceof Error ? e.message : "删除失败");
        } finally {
          setActingId("");
        }
      },
    });
  }
  return (
    <ProjectSettingsFrame
      projectId={projectId}
      activeTab="repositories"
      sectionTitle="仓库"
    >
      {error && (
        <Alert type="error" title={error} showIcon style={{ marginBottom: 16 }} />
      )}
      <Card title="添加仓库" {...cardProps} style={{ ...cardProps.style, marginBottom: 16 }}>
        <Form
          form={form}
          layout="vertical"
          initialValues={{ git_branch: "main", is_default: false }}
          onFinish={add}
        >
          <Form.Item label="名称" name="name" rules={[{ required: true }]}>
            <Input />
          </Form.Item>
          <Form.Item label="Git 地址" name="git_url">
            <Input />
          </Form.Item>
          <Form.Item label="分支" name="git_branch">
            <Input />
          </Form.Item>
          <Form.Item name="is_default" valuePropName="checked">
            <Checkbox>设为默认仓库</Checkbox>
          </Form.Item>
          <Button type="primary" htmlType="submit">
            添加仓库
          </Button>
        </Form>
      </Card>
      <Card title="已绑定仓库" {...cardProps}>
        <Table dataSource={repos} rowKey="id" pagination={false}>
        <Table.Column title="名称" dataIndex="name" />
        <Table.Column title="Git 地址" dataIndex="git_url" />
        <Table.Column title="分支" dataIndex="git_branch" />
        <Table.Column
          title="默认"
          render={(_, row: Repository) => (row.is_default ? "是" : "")}
        />
        <Table.Column
          title="操作"
          render={(_, row: Repository) => (
            <Space>
              <Button danger loading={actingId === row.id} onClick={() => remove(row)}>
                删除
              </Button>
            </Space>
          )}
        />
      </Table>
      </Card>
    </ProjectSettingsFrame>
  );
}
