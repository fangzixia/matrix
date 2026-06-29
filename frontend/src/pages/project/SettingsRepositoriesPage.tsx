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
  async function pull(r: Repository) {
    setError("");
    setMessage("");
    setActingId(r.id);
    try {
      await reposApi.pullRepo(projectId, r.id);
      setMessage(`已拉取 ${r.name}`);
    } catch (e) {
      setError(e instanceof Error ? e.message : "拉取失败");
    } finally {
      setActingId("");
    }
  }
  async function push(r: Repository) {
    setError("");
    setMessage("");
    setActingId(r.id);
    try {
      await reposApi.pushRepo(projectId, r.id);
      setMessage(`已推送 ${r.name}`);
    } catch (e) {
      setError(e instanceof Error ? e.message : "推送失败");
    } finally {
      setActingId("");
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
              <Button loading={actingId === row.id} onClick={() => pull(row)}>
                拉取
              </Button>
              <Button loading={actingId === row.id} onClick={() => push(row)}>
                推送
              </Button>
              <Button danger loading={actingId === row.id} onClick={() => remove(row)}>
                删除
              </Button>
            </Space>
          )}
        />
      </Table>
    </div>
  );
}
