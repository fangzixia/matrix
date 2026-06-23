import { useEffect, useState } from "react";
import { Alert, Button, Form, Input, Space, Spin, Typography } from "antd";
import { getMcpSettings, saveMcpSettings } from "@/api/system";
import { parseMcpServersJson } from "@/utils/mcpSettings";

export default function SystemMCPTab() {
  const [error, setError] = useState("");
  const [message, setMessage] = useState("");
  const [saving, setSaving] = useState(false);
  const [loaded, setLoaded] = useState(false);
  const [mcpJson, setMcpJson] = useState("{}");
  async function load() {
    setError("");
    const s = await getMcpSettings();
    setMcpJson(JSON.stringify(s.mcp_servers || {}, null, 2));
    setLoaded(true);
  }
  useEffect(() => {
    load();
  }, []);
  async function save() {
    setError("");
    setMessage("");
    setSaving(true);
    try {
      const mcp_servers = parseMcpServersJson(mcpJson || "{}");
      const saved = await saveMcpSettings(mcp_servers);
      setMcpJson(JSON.stringify(saved.mcp_servers || {}, null, 2));
      setMessage("MCP 配置已保存");
    } catch (e) {
      setError(e instanceof Error ? e.message : "MCP 保存失败");
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
      <Typography.Title level={4}>MCP 服务</Typography.Title>
      <Typography.Text type="secondary">
        支持直接粘贴 Cursor 的 <code>mcp.json</code>（含 <code>mcpServers</code>{" "}
        包装）或裸服务对象 JSON。
      </Typography.Text>
      <Form layout="vertical">
        <Form.Item label="MCP 服务配置">
          <Input.TextArea
            rows={16}
            value={mcpJson}
            onChange={(e) => setMcpJson(e.target.value)}
            spellCheck={false}
          />
        </Form.Item>
      </Form>
      <Button type="primary" loading={saving} onClick={save}>
        保存 MCP 配置
      </Button>
    </Space>
  );
}
