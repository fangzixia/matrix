import { useEffect, useState } from "react";
import { useParams } from "react-router-dom";
import {
  Alert,
  Button,
  Card,
  Checkbox,
  Form,
  Input,
  Space,
  Table,
  Tabs,
  Typography,
} from "antd";
import * as reposApi from "@/api/repositories";
import type { Repository } from "@/api/repositories";
import { useSettingsTabNavigate } from "@/hooks/useSettingsTabNavigate";
import { settingsTabs } from "@/locales/zh-CN";

export default function SettingsRepositoriesPage() {
  const { id: projectId = "" } = useParams();
  const onSettingsTabChange = useSettingsTabNavigate(projectId);
  const [repos, setRepos] = useState<Repository[]>([]);
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");
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
  async function pull(r: Repository) {
    setMessage("");
    await reposApi.pullRepo(projectId, r.id);
    setMessage(`已拉取 ${r.name}`);
  }
  async function push(r: Repository) {
    setMessage("");
    await reposApi.pushRepo(projectId, r.id);
    setMessage(`已推送 ${r.name}`);
  }
  async function remove(r: Repository) {
    await reposApi.deleteRepository(projectId, r.id);
    await load();
  }
  return (
    <div>
      <Tabs
        activeKey="repositories"
        onChange={onSettingsTabChange}
        items={settingsTabs(projectId).map((tab) => ({
          key: tab.key,
          label: tab.label,
        }))}
        style={{ marginBottom: 16 }}
      />
      <Typography.Title level={2}>仓库</Typography.Title>
      {error && (
        <Alert type="error" message={error} style={{ marginBottom: 16 }} />
      )}
      {message && (
        <Alert type="success" message={message} style={{ marginBottom: 16 }} />
      )}
      <Card style={{ maxWidth: 560, marginBottom: 24 }}>
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
              <Button onClick={() => pull(row)}>拉取</Button>
              <Button onClick={() => push(row)}>推送</Button>
              <Button danger onClick={() => remove(row)}>
                删除
              </Button>
            </Space>
          )}
        />
      </Table>
    </div>
  );
}
