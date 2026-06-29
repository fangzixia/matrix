import { useCallback, useEffect, useMemo, useState } from "react";
import { Link, useLocation, useNavigate, useParams } from "react-router-dom";
import {
  Alert,
  Breadcrumb,
  Button,
  Card,
  Drawer,
  Flex,
  Input,
  Space,
  Splitter,
  Table,
  Tag,
  Typography,
  theme,
} from "antd";

import * as runsApi from "@/api/runs";
import * as projectsApi from "@/api/projects";
import PlanComposePanel from "@/components/ai/PlanComposePanel";
import type { ChatPromptItem } from "@/components/ai/MatrixAiChat";
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

const PLAN_PROMPTS: ChatPromptItem[] = [
  {
    key: "write",
    label: "基于代码库编写实现计划",
    description: "调研源码并输出含验收标准的计划文档",
  },
  {
    key: "acceptance",
    label: "补充验收标准与风险项",
    description: "完善计划中的范围、风险与澄清项",
  },
  {
    key: "review",
    label: "审查现有计划文档",
    description: "检查计划完整性、一致性与可执行性",
  },
];

function statusColor(status: string) {
  if (status === "succeeded") return "success";
  if (status === "failed" || status === "cancelled") return "error";
  if (status === "running") return "processing";
  return "default";
}

function PlanFileFooter({
  filePath,
  planOptions,
  onFilePathChange,
  onPreview,
}: {
  filePath: string;
  planOptions: DocFileOption[];
  onFilePathChange: (path: string) => void;
  onPreview: (path: string) => void;
}) {
  if (planOptions.length > 0) {
    return (
      <Space.Compact style={{ width: "100%" }}>
        <DocFileSelect
          options={planOptions}
          value={filePath}
          onChange={onFilePathChange}
          placeholder="选择计划文件（可选）"
        />
        {filePath && (
          <Button onClick={() => onPreview(filePath)}>预览</Button>
        )}
      </Space.Compact>
    );
  }
  return (
    <Input
      value={filePath}
      onChange={(e) => onFilePathChange(e.target.value)}
      placeholder="计划文件路径（可选，如 docs/plans/PLAN-*.md）"
    />
  );
}

function TaskHistoryTable({
  projectId,
  kind,
  tasks,
}: {
  projectId: string;
  kind: string;
  tasks: runsApi.Run[];
}) {
  return (
    <Table
      dataSource={tasks}
      rowKey="id"
      pagination={{ pageSize: 10 }}
      size="small"
    >
      <Table.Column
        title="标题"
        render={(_, row: runsApi.Run) => (
          <Link to={`/projects/${projectId}/${kind}/${row.id}`}>
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
  );
}

function PreviewDrawer({
  preview,
  onClose,
}: {
  preview: { title: string; content: string; path: string } | null;
  onClose: () => void;
}) {
  return (
    <Drawer
      className="doc-preview-drawer"
      title={preview?.title}
      open={!!preview}
      onClose={onClose}
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
  );
}

export default function StagePage() {
  const { id = "" } = useParams();
  const location = useLocation();
  const navigate = useNavigate();
  const { token } = theme.useToken();
  const headerHeight = token.Layout?.headerHeight ?? 48;
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
  const [docsError, setDocsError] = useState("");
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
    setDocsError("");
    projectsApi
      .listPlans(id)
      .then((res) => {
        setPlans(res.plans ?? []);
      })
      .catch((e) =>
        setDocsError(e instanceof Error ? e.message : "计划文档加载失败"),
      );
  }, [id]);

  useEffect(() => {
    if (kind !== "build") return;
    projectsApi
      .listEvaluations(id)
      .then((res) => {
        setEvaluations(res.evaluations ?? []);
      })
      .catch((e) =>
        setDocsError(e instanceof Error ? e.message : "评测报告加载失败"),
      );
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

  const needsPlanFile =
    kind === "implement" || kind === "verify" || kind === "build";
  const requiresApprovedPlan =
    kind === "implement" || kind === "verify" || kind === "build";

  const refreshPlanData = useCallback(() => {
    fetchRuns(id, "plan");
    projectsApi
      .listPlans(id)
      .then((res) => setPlans(res.plans ?? []))
      .catch((e) =>
        setDocsError(e instanceof Error ? e.message : "计划文档加载失败"),
      );
  }, [fetchRuns, id]);

  if (kind === "plan") {
    return (
      <>
        {docsError && (
          <Alert
            type="warning"
            showIcon
            message={docsError}
            style={{ margin: "0 24px 12px" }}
          />
        )}
        <Splitter
          vertical
          style={{
            height: `calc(100vh - ${headerHeight}px)`,
            minHeight: 0,
          }}
        >
          <Splitter.Panel defaultSize="72%" min="240px">
            <PlanComposePanel
              projectId={id}
              filePath={filePath}
              prompts={PLAN_PROMPTS}
              welcomeTitle="编写计划"
              welcomeDescription={harnessKindHints.plan}
              onRunComplete={refreshPlanData}
              footer={
                <PlanFileFooter
                  filePath={filePath}
                  planOptions={planOptions}
                  onFilePathChange={setFilePath}
                  onPreview={(path) => openPreview(path, plans)}
                />
              }
            />
          </Splitter.Panel>
          <Splitter.Panel min="160px">
            <Flex
              vertical
              style={{
                height: "100%",
                minHeight: 0,
                padding: "12px 24px 16px",
                background: token.colorBgContainer,
                boxSizing: "border-box",
              }}
            >
              <Typography.Text
                strong
                style={{ display: "block", marginBottom: 8, flexShrink: 0 }}
              >
                历史任务
              </Typography.Text>
              <div style={{ flex: 1, minHeight: 0, overflow: "auto" }}>
                <TaskHistoryTable
                  projectId={id}
                  kind={kind}
                  tasks={stageTasks}
                />
              </div>
            </Flex>
          </Splitter.Panel>
        </Splitter>
        <PreviewDrawer preview={preview} onClose={() => setPreview(null)} />
      </>
    );
  }

  return (
    <div>
      <Breadcrumb
        style={{ marginBottom: 16 }}
        items={[{ title: <Link to={`/projects/${id}`}>概览</Link> }, { title }]}
      />
      <Typography.Title level={2}>{title}</Typography.Title>
      {docsError && (
        <Alert
          type="warning"
          showIcon
          message={docsError}
          style={{ marginBottom: 16 }}
        />
      )}
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
                  placeholder="选择计划文件（必填）"
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
                placeholder="计划文件路径（必填）"
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
        </Space>
      </Card>
      <TaskHistoryTable projectId={id} kind={kind} tasks={stageTasks} />
      <PreviewDrawer preview={preview} onClose={() => setPreview(null)} />
    </div>
  );
}
