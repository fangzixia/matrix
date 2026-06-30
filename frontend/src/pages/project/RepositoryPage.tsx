import { useEffect, useState } from "react";
import { useParams, useSearchParams } from "react-router-dom";
import {
  Alert,
  Breadcrumb,
  Button,
  Card,
  Empty,
  Flex,
  List,
  Row,
  Col,
  Typography,
} from "antd";
import { FileOutlined, FolderOutlined, HomeOutlined } from "@ant-design/icons";
import * as projectsApi from "@/api/projects";
import type { FileEntry } from "@/api/projects";

export default function RepositoryPage() {
  const { id = "" } = useParams();
  const [searchParams] = useSearchParams();
  const runId = searchParams.get("run_id") ?? "";
  const [path, setPath] = useState("");
  const [files, setFiles] = useState<FileEntry[]>([]);
  const [content, setContent] = useState("");
  const [selected, setSelected] = useState("");
  const [error, setError] = useState("");

  async function loadDir(p = "") {
    if (!runId) return;
    setPath(p);
    setError("");
    try {
      const res = await projectsApi.listFiles(id, runId, p);
      setFiles(res.files);
      setContent("");
      setSelected("");
    } catch (e) {
      setError(e instanceof Error ? e.message : "加载失败");
    }
  }

  async function openFile(file: FileEntry) {
    if (file.is_dir) {
      await loadDir(file.path);
      return;
    }
    setSelected(file.path);
    try {
      const res = await projectsApi.readFile(id, runId, file.path);
      setContent(res.content);
    } catch (e) {
      setError(e instanceof Error ? e.message : "读取失败");
    }
  }

  useEffect(() => {
    if (runId) {
      loadDir();
    }
  }, [id, runId]);

  const pathParts = path ? path.split("/").filter(Boolean) : [];

  if (!runId) {
    return (
      <Alert
        type="info"
        showIcon
        message="请从任务详情页打开仓库浏览"
        description="仓库按 Run 独立存储，URL 需携带 run_id 参数，例如 /projects/{id}/repository?run_id=..."
      />
    );
  }

  return (
    <>
      <Flex
        justify="space-between"
        align="center"
        wrap="wrap"
        gap={12}
        style={{ marginBottom: 12 }}
      >
        <Typography.Title level={2} style={{ margin: 0 }}>
          仓库
        </Typography.Title>
        <Typography.Text type="secondary">Run: {runId}</Typography.Text>
      </Flex>
      {error && (
        <Alert type="error" message={error} style={{ marginBottom: 12 }} />
      )}
      <Row gutter={16}>
        <Col xs={24} md={8}>
          <Card>
            <Breadcrumb
              style={{ marginBottom: 8 }}
              items={[
                {
                  title: (
                    <Button
                      type="link"
                      size="small"
                      icon={<HomeOutlined />}
                      onClick={() => loadDir("")}
                      style={{ padding: 0 }}
                    >
                      根目录
                    </Button>
                  ),
                },
                ...pathParts.map((part, index) => ({
                  title: (
                    <Button
                      type="link"
                      size="small"
                      style={{ padding: 0 }}
                      onClick={() =>
                        loadDir(pathParts.slice(0, index + 1).join("/"))
                      }
                    >
                      {part}
                    </Button>
                  ),
                })),
              ]}
            />
            <List
              dataSource={files}
              locale={{
                emptyText: (
                  <Empty
                    image={Empty.PRESENTED_IMAGE_SIMPLE}
                    description="空目录"
                  />
                ),
              }}
              renderItem={(f) => (
                <List.Item style={{ padding: "4px 0" }}>
                  <Button
                    type="link"
                    icon={f.is_dir ? <FolderOutlined /> : <FileOutlined />}
                    onClick={() => openFile(f)}
                    style={{ padding: 0, height: "auto" }}
                  >
                    {f.name}
                  </Button>
                </List.Item>
              )}
            />
          </Card>
        </Col>
        <Col xs={24} md={16}>
          <Card>
            {selected && (
              <Typography.Title level={5}>{selected}</Typography.Title>
            )}
            {content ? (
              <Typography.Paragraph>
                <pre
                  style={{
                    whiteSpace: "pre-wrap",
                    wordBreak: "break-word",
                    fontSize: 12,
                    margin: 0,
                  }}
                >
                  {content}
                </pre>
              </Typography.Paragraph>
            ) : (
              <Empty description="选择文件查看内容" />
            )}
          </Card>
        </Col>
      </Row>
    </>
  );
}
