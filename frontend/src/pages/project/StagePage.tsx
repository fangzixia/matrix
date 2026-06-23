import { useEffect, useMemo, useState } from "react";
import { Link, useLocation, useNavigate, useParams } from "react-router-dom";
import {
  Alert,
  Breadcrumb,
  Button,
  Card,
  Drawer,
  Input,
  Space,
  Table,
  Tag,
  Typography,
} from "antd";

import * as runsApi from "@/api/runs";
import * as projectsApi from "@/api/projects";
import { useRunStore } from "@/stores/run";
import {
  harnessKindHints,
  runKindLabels,
  runStatusLabels,
  stageTitles,
} from "@/locales/zh-CN";

import { isStageKind, stageKindFromPath } from "@/utils/stage";
import DocFileSelect, {
  type DocFileOption,
} from "@/components/docs/DocFileSelect";

import MarkdownView from "@/components/docs/MarkdownView";

function statusColor(status: string) {
  if (status === "succeeded") return "success";
  if (status === "failed" || status === "cancelled") return "error";
  if (status === "running") return "processing";
  return "default";
}

export default function StagePage() {
  const { id = "" } = useParams();
  const location = useLocation();
  const navigate = useNavigate();
  const pathKind = stageKindFromPath(location.pathname);
  const kind = pathKind && isStageKind(pathKind) ? pathKind : "plan";
  const runs = useRunStore((s) => s.runs);
  const fetchRuns = useRunStore((s) => s.fetchRuns);
  const [message, setMessage] = useState("");
  const [filePath, setFilePath] = useState("");
  const [evalPath, setEvalPath] = useState("");
  const [plans, setPlans] = useState<projectsApi.PlanItem[]>([]);
  const [evaluations, setEvaluations] = useState<projectsApi.EvaluationItem[]>(
    [],
  );
  const [starting, setStarting] = useState(false);
  const [startError, setStartError] = useState("");
  const [preview, setPreview] = useState<{
    title: string;
    content: string;
    path: string;
  } | null>(null);
  useEffect(() => {
    fetchRuns(id, kind);
  }, [id, kind, fetchRuns]);
  useEffect(() => {
    projectsApi
      .listPlans(id)
      .then((res) => {
        setPlans(res.plans ?? []);
      })
      .catch(() => setPlans([]));
  }, [id]);
  useEffect(() => {
    if (kind !== "build") return;
    projectsApi
      .listEvaluations(id)
      .then((res) => {
        setEvaluations(res.evaluations ?? []);
      })
      .catch(() => setEvaluations([]));
  }, [id, kind]);
  const kindHint = harnessKindHints[kind];
  const title = stageTitles[kind] || kind;
  const planOptions: DocFileOption[] = useMemo(
    () =>
      plans.map((r) => ({
        value: r.path,
        label: r.title || r.path,
        path: r.path,
        content: r.content,
      })),
    [plans],
  );
  const evalOptions: DocFileOption[] = useMemo(
    () =>
      evaluations.map((r) => ({
        value: r.path,
        label: r.title || r.path,
        path: r.path,
        content: r.content,
      })),
    [evaluations],
  );
  const stageTasks = useMemo(
    () => runs.filter((r) => r.kind === kind),
    [runs, kind],
  );
  function openPreview(
    path: string,
    items: Array<{ path: string; title?: string; content?: string }>,
  ) {
    const item = items.find((i) => i.path === path);
    if (!item) return;
    setPreview({
      title: item.title || item.path,
      content: item.content || "（无内容）",
      path: item.path,
    });
  }
  async function startTask() {
    setStartError("");
    if (
      (kind === "implement" || kind === "verify" || kind === "build") &&
      !filePath
    ) {
      setStartError("请选择计划文件");
      return;
    }
    if (requiresApprovedPlan && filePath) {
      const selected = plans.find((p) => p.path === filePath);
      if (selected && selected.status !== "approved") {
        setStartError("计划尚未批准，请先在概览页确认计划");
        return;
      }
    }
    setStarting(true);
    try {
      const taskMessage = message || `${runKindLabels[kind]}任务`;
      const run = await runsApi.startRun(
        id,
        taskMessage,
        kind,
        filePath,
        kind === "build" ? evalPath : "",
      );
      setMessage("");
      setEvalPath("");
      navigate(`/projects/${id}/${kind}/${run.id}`);
    } catch (e) {
      setStartError(e instanceof Error ? e.message : "启动失败");
    } finally {
      setStarting(false);
    }
  }
  async function startPipelineTask() {
    setStartError("");
    if (!filePath) {
      setStartError("流水线需要选择已批准的计划文件");
      return;
    }
    const selected = plans.find((p) => p.path === filePath);
    if (selected && selected.status !== "approved") {
      setStartError("计划尚未批准，请先在概览页确认计划");
      return;
    }
    setStarting(true);
    try {
      const taskMessage = message || "流水线任务";
      const run = await runsApi.startPipeline(id, taskMessage, filePath);
      setMessage("");
      navigate(`/projects/${id}/runs/${run.id}`);
    } catch (e) {
      setStartError(e instanceof Error ? e.message : "启动失败");
    } finally {
      setStarting(false);
    }
  }
  const needsPlanFile =
    kind === "plan" ||
    kind === "implement" ||
    kind === "verify" ||
    kind === "build";
  const requiresApprovedPlan =
    kind === "implement" || kind === "verify" || kind === "build";
  return (
    <div>
      <Breadcrumb
        style={{ marginBottom: 16 }}
        items={[{ title: <Link to={`/projects/${id}`}>概览</Link> }, { title }]}
      />
      <Typography.Title level={2}>{title}</Typography.Title>
      <Card style={{ marginBottom: 16 }}>
        <Space direction="vertical" style={{ width: "100%" }} size="middle">
          {kindHint && <Alert type="info" showIcon message={kindHint} />}
          {startError && <Alert type="error" showIcon message={startError} />}
          <Input
            value={message}
            onChange={(e) => setMessage(e.target.value)}
            placeholder="任务描述"
          />
          {needsPlanFile &&
            (planOptions.length > 0 ? (
              <Space.Compact style={{ width: "100%" }}>
                <DocFileSelect
                  options={planOptions}
                  value={filePath}
                  onChange={setFilePath}
                  placeholder={
                    kind === "plan"
                      ? "选择计划文件（可选）"
                      : "选择计划文件（必填）"
                  }
                />
                {filePath && (
                  <Button onClick={() => openPreview(filePath, plans)}>
                    预览
                  </Button>
                )}
              </Space.Compact>
            ) : (
              <Input
                value={filePath}
                onChange={(e) => setFilePath(e.target.value)}
                placeholder={
                  kind === "plan"
                    ? "计划文件路径（可选，如 docs/plans/PLAN-*.md）"
                    : "计划文件路径（必填）"
                }
              />
            ))}
          {kind === "build" && evalOptions.length > 0 && (
            <Space.Compact style={{ width: "100%" }}>
              <DocFileSelect
                options={evalOptions}
                value={evalPath}
                onChange={setEvalPath}
                placeholder="选择基准评测报告（可选）"
              />
              {evalPath && (
                <Button onClick={() => openPreview(evalPath, evaluations)}>
                  预览
                </Button>
              )}
            </Space.Compact>
          )}
          <Button type="primary" loading={starting} onClick={startTask}>
            启动
          </Button>
          {kind === "plan" && (
            <Button loading={starting} onClick={startPipelineTask}>
              启动流水线（计划 → 构建）
            </Button>
          )}
        </Space>
      </Card>
      <Table dataSource={stageTasks} rowKey="id" pagination={{ pageSize: 20 }}>
        <Table.Column
          title="标题"
          render={(_, row: runsApi.Run) => (
            <Link to={`/projects/${id}/${kind}/${row.id}`}>
              {row.title || row.id}
            </Link>
          )}
        />
        <Table.Column
          title="状态"
          render={(_, row: runsApi.Run) => (
            <Tag color={statusColor(row.status)}>
              {runStatusLabels[row.status] || row.status}
            </Tag>
          )}
        />
        <Table.Column
          title="创建时间"
          render={(_, row: runsApi.Run) =>
            new Date(row.created_at).toLocaleString("zh-CN")
          }
        />
      </Table>
      <Drawer
        className="doc-preview-drawer"
        title={preview?.title}
        open={!!preview}
        onClose={() => setPreview(null)}
        width={720}
      >
        {preview && (
          <>
            <Typography.Text
              type="secondary"
              style={{ display: "block", marginBottom: 12, fontSize: 12 }}
            >
              {preview.path}
            </Typography.Text>
            <MarkdownView content={preview.content} />
          </>
        )}
      </Drawer>
    </div>
  );
}
