import { useEffect, useState } from "react";
import {
  Alert,
  Button,
  Card,
  Checkbox,
  Form,
  Input,
  InputNumber,
  Radio,
  Space,
  Spin,
  Typography,
} from "antd";
import {
  getAISettings,
  saveAISettings,
  type AISettings,
  type ModelProfile,
} from "@/api/system";

export default function SystemAITab() {
  const [error, setError] = useState("");
  const [message, setMessage] = useState("");
  const [saving, setSaving] = useState(false);
  const [loaded, setLoaded] = useState(false);
  const [form, setForm] = useState<AISettings>({
    models: [],
    context: { auto_compact_threshold: 100000, keep_recent_messages: 8 },
    security: {
      allow_shell: false,
      allow_command_mcp: false,
      shell_timeout: "60s",
    },
  });
  async function load() {
    setError("");
    const s = await getAISettings();
    setForm({ ...s, models: s.models?.length ? s.models : [] });
    setLoaded(true);
  }
  useEffect(() => {
    load();
  }, []);
  function newModel(): ModelProfile {
    const isFirst = form.models.length === 0;
    return {
      id: crypto.randomUUID(),
      name: "",
      base_url: "",
      model: "",
      max_tokens: 8192,
      enabled: true,
      default: isFirst,
      multimodal: false,
      attachment_types: [],
    };
  }
  function setDefaultModel(id: string) {
    setForm((f) => ({
      ...f,
      models: f.models.map((m) => ({
        ...m,
        default: m.id === id,
        enabled: m.id === id ? true : m.enabled,
      })),
    }));
  }
  async function save() {
    setError("");
    setMessage("");
    for (const m of form.models) {
      if (m.multimodal && (!m.attachment_types || m.attachment_types.length === 0)) {
        setError(`模型「${m.name || m.model || "未命名"}」已启用多模态，请至少选择一种附件类型`);
        return;
      }
    }
    setSaving(true);
    try {
      const saved = await saveAISettings(form);
      setForm({ ...saved, models: saved.models?.length ? saved.models : [] });
      setMessage("模型配置已保存");
    } catch (e) {
      setError(e instanceof Error ? e.message : "保存失败");
    } finally {
      setSaving(false);
    }
  }
  if (!loaded)
    return <Spin style={{ display: "block", margin: "24px auto" }} />;
  return (
    <Space orientation="vertical" size="middle" style={{ width: "100%" }}>
      {error && <Alert type="error" title={error} />}
      {message && <Alert type="success" title={message} />}
      <Typography.Title level={4}>模型配置</Typography.Title>
      <Typography.Text type="secondary">
        可配置多个模型并选择启用；标记为「默认」的已启用模型将用于系统 Run。
      </Typography.Text>
      {form.models.map((row, index) => (
        <Card key={row.id} size="small">
          <Space orientation="vertical" style={{ width: "100%" }} size="middle">
            <Space wrap>
              <Checkbox
                checked={row.enabled}
                onChange={(e) => {
                  const models = [...form.models];
                  models[index] = { ...row, enabled: e.target.checked };
                  setForm({ ...form, models });
                }}
              >
                启用
              </Checkbox>
              <Radio
                checked={row.default}
                onChange={() => setDefaultModel(row.id)}
              >
                默认
              </Radio>
              <Button
                danger
                onClick={() => {
                  const models = form.models.filter((_, i) => i !== index);
                  if (row.default && models.length > 0) {
                    const first = models.find((m) => m.enabled) || models[0];
                    models.forEach((m) => {
                      m.default = m.id === first.id;
                    });
                  }
                  setForm({ ...form, models });
                }}
              >
                删除
              </Button>
            </Space>
            <Form layout="vertical">
              <Form.Item label="显示名称">
                <Input
                  value={row.name}
                  onChange={(e) => {
                    const models = [...form.models];
                    models[index] = { ...row, name: e.target.value };
                    setForm({ ...form, models });
                  }}
                  placeholder="DeepSeek V4"
                />
              </Form.Item>
              <Form.Item label="API 地址">
                <Input
                  value={row.base_url}
                  onChange={(e) => {
                    const models = [...form.models];
                    models[index] = { ...row, base_url: e.target.value };
                    setForm({ ...form, models });
                  }}
                />
              </Form.Item>
              <Form.Item label="API Key">
                <Input.Password
                  value={row.api_key}
                  onChange={(e) => {
                    const models = [...form.models];
                    models[index] = { ...row, api_key: e.target.value };
                    setForm({ ...form, models });
                  }}
                  placeholder={
                    row.api_key_set ? "已配置，留空则不修改" : "未配置"
                  }
                />
              </Form.Item>
              <Form.Item label="模型名称">
                <Input
                  value={row.model}
                  onChange={(e) => {
                    const models = [...form.models];
                    models[index] = { ...row, model: e.target.value };
                    setForm({ ...form, models });
                  }}
                />
              </Form.Item>
              <Form.Item label="最大 Token">
                <InputNumber
                  min={1}
                  value={row.max_tokens}
                  onChange={(v) => {
                    const models = [...form.models];
                    models[index] = { ...row, max_tokens: v ?? 8192 };
                    setForm({ ...form, models });
                  }}
                  style={{ width: "100%" }}
                />
              </Form.Item>
              <Form.Item label="多模态">
                <Space orientation="vertical">
                  <Checkbox
                    checked={row.multimodal ?? false}
                    onChange={(e) => {
                      const models = [...form.models];
                      models[index] = {
                        ...row,
                        multimodal: e.target.checked,
                        attachment_types: e.target.checked
                          ? row.attachment_types?.length
                            ? row.attachment_types
                            : ["image"]
                          : [],
                      };
                      setForm({ ...form, models });
                    }}
                  >
                    支持多模态
                  </Checkbox>
                  {row.multimodal && (
                    <Checkbox.Group
                      value={row.attachment_types ?? []}
                      options={[
                        { label: "图片", value: "image" },
                        { label: "文档（txt / md）", value: "document" },
                      ]}
                      onChange={(values) => {
                        const models = [...form.models];
                        models[index] = {
                          ...row,
                          attachment_types: values as string[],
                        };
                        setForm({ ...form, models });
                      }}
                    />
                  )}
                </Space>
              </Form.Item>
            </Form>
          </Space>
        </Card>
      ))}
      <Button
        onClick={() =>
          setForm({ ...form, models: [...form.models, newModel()] })
        }
      >
        添加模型
      </Button>
      <h3>上下文</h3>
      <Form layout="vertical">
        <Form.Item label="自动压缩阈值">
          <InputNumber
            value={form.context.auto_compact_threshold}
            onChange={(v) =>
              setForm({
                ...form,
                context: {
                  ...form.context,
                  auto_compact_threshold: v ?? 100000,
                },
              })
            }
            style={{ width: "100%" }}
          />
        </Form.Item>
        <Form.Item label="保留最近消息数">
          <InputNumber
            value={form.context.keep_recent_messages}
            onChange={(v) =>
              setForm({
                ...form,
                context: { ...form.context, keep_recent_messages: v ?? 8 },
              })
            }
            style={{ width: "100%" }}
          />
        </Form.Item>
      </Form>
      <h3>安全</h3>
      <Space orientation="vertical">
        <Checkbox
          checked={form.security.allow_shell}
          onChange={(e) =>
            setForm({
              ...form,
              security: { ...form.security, allow_shell: e.target.checked },
            })
          }
        >
          允许 Shell
        </Checkbox>
        <Checkbox
          checked={form.security.allow_command_mcp}
          onChange={(e) =>
            setForm({
              ...form,
              security: {
                ...form.security,
                allow_command_mcp: e.target.checked,
              },
            })
          }
        >
          允许命令型 MCP
        </Checkbox>
        <Form.Item label="Shell 超时">
          <Input
            value={form.security.shell_timeout}
            onChange={(e) =>
              setForm({
                ...form,
                security: { ...form.security, shell_timeout: e.target.value },
              })
            }
            placeholder="60s"
          />
        </Form.Item>
      </Space>
      <Button type="primary" loading={saving} onClick={save}>
        保存模型配置
      </Button>
    </Space>
  );
}
