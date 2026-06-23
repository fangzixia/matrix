import { useEffect, useState } from "react";
import {
  Alert,
  Button,
  Checkbox,
  Form,
  Input,
  InputNumber,
  Space,
  Spin,
  Typography,
} from "antd";
import {
  getWorkerSettings,
  saveWorkerSettings,
  type SystemWorkerSettings,
} from "@/api/system";

export default function SystemWorkerTab() {
  const [error, setError] = useState("");
  const [message, setMessage] = useState("");
  const [saving, setSaving] = useState(false);
  const [loaded, setLoaded] = useState(false);
  const [form, setForm] = useState<SystemWorkerSettings>({
    enabled: true,
    poll_interval: "2s",
    max_attempts: 3,
    concurrency: 2,
  });
  useEffect(() => {
    getWorkerSettings().then((s) => {
      setForm(s);
      setLoaded(true);
    });
  }, []);
  async function save() {
    setError("");
    setMessage("");
    setSaving(true);
    try {
      setForm(await saveWorkerSettings(form));
      setMessage("并发配置已保存。并发数变更需重启服务后完全生效。");
    } catch (e) {
      setError(e instanceof Error ? e.message : "保存失败");
    } finally {
      setSaving(false);
    }
  }
  if (!loaded)
    return <Spin style={{ display: "block", margin: "24px auto" }} />;
  return (
    <Space direction="vertical" size="middle" style={{ width: "100%" }}>
      {error && <Alert type="error" message={error} />}
      {message && <Alert type="success" message={message} />}
      <Typography.Title level={4}>并发控制</Typography.Title>
      <Typography.Text type="secondary">
        控制嵌入式任务 Worker 的轮询与并发。修改并发数后建议重启 Web 服务。
      </Typography.Text>
      <Form layout="vertical">
        <Form.Item label="启用 Worker">
          <Checkbox
            checked={form.enabled}
            onChange={(e) => setForm({ ...form, enabled: e.target.checked })}
          >
            启用
          </Checkbox>
        </Form.Item>
        <Form.Item label="轮询间隔">
          <Input
            value={form.poll_interval}
            onChange={(e) =>
              setForm({ ...form, poll_interval: e.target.value })
            }
            placeholder="2s"
          />
        </Form.Item>
        <Form.Item label="最大重试次数">
          <InputNumber
            min={1}
            value={form.max_attempts}
            onChange={(v) => setForm({ ...form, max_attempts: v ?? 3 })}
            style={{ width: "100%" }}
          />
        </Form.Item>
        <Form.Item label="并发数">
          <InputNumber
            min={1}
            value={form.concurrency}
            onChange={(v) => setForm({ ...form, concurrency: v ?? 2 })}
            style={{ width: "100%" }}
          />
        </Form.Item>
      </Form>
      <Button type="primary" loading={saving} onClick={save}>
        保存并发配置
      </Button>
    </Space>
  );
}
