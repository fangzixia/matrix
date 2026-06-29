import { useEffect, useState } from "react";
import {
  Alert,
  Button,
  Card,
  Form,
  Input,
  Space,
  Spin,
  Typography,
} from "antd";
import {
  getGitSettings,
  saveGitSettings,
  testGitAccess,
  type GitAccess,
  type SystemGitSettings,
} from "@/api/system";

export default function SystemGitTab() {
  const [error, setError] = useState("");
  const [message, setMessage] = useState("");
  const [saving, setSaving] = useState(false);
  const [loaded, setLoaded] = useState(false);
  const [gitTestUrl, setGitTestUrl] = useState("");
  const [gitTestMsg, setGitTestMsg] = useState("");
  const [gitTestError, setGitTestError] = useState("");
  const [gitTesting, setGitTesting] = useState(false);
  const [form, setForm] = useState<SystemGitSettings>({
    clone_timeout: "300s",
    accesses: [],
  });
  async function load() {
    setError("");
    const s = await getGitSettings();
    setForm({ ...s, accesses: s.accesses?.length ? s.accesses : [] });
    setLoaded(true);
  }
  useEffect(() => {
    load();
  }, []);
  function newGitAccess(): GitAccess {
    return {
      id: crypto.randomUUID(),
      name: "",
      host: "*",
      ssh_key_path: form.default_ssh_key_path || "",
    };
  }
  async function save() {
    setError("");
    setMessage("");
    setSaving(true);
    try {
      const saved = await saveGitSettings(form);
      setForm({
        ...saved,
        accesses: saved.accesses?.length ? saved.accesses : [],
      });
      setMessage("Git 配置已保存");
    } catch (e) {
      setError(e instanceof Error ? e.message : "保存失败");
    } finally {
      setSaving(false);
    }
  }
  async function runGitTest() {
    setGitTestError("");
    setGitTestMsg("");
    setGitTesting(true);
    try {
      const res = await testGitAccess(gitTestUrl.trim());
      setGitTestMsg(res.message);
    } catch (e) {
      setGitTestError(e instanceof Error ? e.message : "测试失败");
    } finally {
      setGitTesting(false);
    }
  }
  if (!loaded)
    return <Spin style={{ display: "block", margin: "24px auto" }} />;
  const platformHint = form.default_ssh_key_path
    ? `服务运行于 ${form.platform_label || "当前系统"}，默认私钥路径：${form.default_ssh_key_path}`
    : `服务运行于 ${form.platform_label || "当前系统"}，请填写私钥绝对路径`;
  return (
    <Space direction="vertical" size="middle" style={{ width: "100%" }}>
      {error && <Alert type="error" message={error} />}
      {message && <Alert type="success" message={message} />}
      <Typography.Title level={4}>Git 访问</Typography.Title>
      <Typography.Text type="secondary">
        按 Git 主机配置 SSH 私钥，支持多条。
      </Typography.Text>
      <Alert type="info" message={platformHint} />
      <Form layout="vertical">
        <Form.Item label="克隆超时">
          <Input
            value={form.clone_timeout}
            onChange={(e) =>
              setForm({ ...form, clone_timeout: e.target.value })
            }
            placeholder="300s"
          />
        </Form.Item>
      </Form>
      {form.accesses.map((row, index) => (
        <Card key={row.id} size="small">
          <Space direction="vertical" style={{ width: "100%" }}>
            <Form layout="vertical">
              <Form.Item label="名称">
                <Input
                  value={row.name}
                  onChange={(e) => {
                    const accesses = [...form.accesses];
                    accesses[index] = { ...row, name: e.target.value };
                    setForm({ ...form, accesses });
                  }}
                />
              </Form.Item>
              <Form.Item label="主机">
                <Input
                  value={row.host}
                  onChange={(e) => {
                    const accesses = [...form.accesses];
                    accesses[index] = { ...row, host: e.target.value };
                    setForm({ ...form, accesses });
                  }}
                  placeholder="gitlab.example.com 或 *"
                />
              </Form.Item>
              <Form.Item label="SSH 私钥路径">
                <Input
                  value={row.ssh_key_path}
                  onChange={(e) => {
                    const accesses = [...form.accesses];
                    accesses[index] = { ...row, ssh_key_path: e.target.value };
                    setForm({ ...form, accesses });
                  }}
                  placeholder={form.default_ssh_key_path || "~/.ssh/id_rsa"}
                />
              </Form.Item>
            </Form>
            <Button
              danger
              onClick={() =>
                setForm({
                  ...form,
                  accesses: form.accesses.filter((_, i) => i !== index),
                })
              }
            >
              删除
            </Button>
          </Space>
        </Card>
      ))}
      <Button
        onClick={() =>
          setForm({ ...form, accesses: [...form.accesses, newGitAccess()] })
        }
      >
        添加 Git 访问配置
      </Button>
      <Card title="连接测试" size="small">
        <Space direction="vertical" style={{ width: "100%" }}>
          <Input
            value={gitTestUrl}
            onChange={(e) => setGitTestUrl(e.target.value)}
            placeholder="git@github.com:org/repo.git"
          />
          <Button
            loading={gitTesting}
            disabled={!gitTestUrl.trim()}
            onClick={runGitTest}
          >
            测试连接
          </Button>
          {gitTestError && <Alert type="error" message={gitTestError} />}
          {gitTestMsg && <Alert type="success" message={gitTestMsg} />}
          <Typography.Text type="secondary">
            测试会使用当前已保存的 Git 配置；如修改了 SSH 私钥路径，请先保存配置。
          </Typography.Text>
        </Space>
      </Card>
      <Button type="primary" loading={saving} onClick={save}>
        保存 Git 配置
      </Button>
    </Space>
  );
}
