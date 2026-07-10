import { useEffect, useMemo, useRef } from "react";
import { Link, useLocation, useNavigate, useParams } from "react-router-dom";
import {
  Alert,
  Breadcrumb,
  Button,
  Card,
  Descriptions,
  Empty,
  Flex,
  Result,
  Space,
  Spin,
  Statistic,
  Tag,
  Typography,
} from "antd";
import RunActivityPanel from "@/components/ai/RunActivityPanel";
import { runStatusPollIntervalMs } from "@/api/runView";
import * as runsApi from "@/api/runs";
import type { RunViewState } from "@/types/runView";
import { runStatusLabels, stageTitles } from "@/locales/zh-CN";
import { isStageKind, stageKindFromPath } from "@/utils/stage";
import { useRunStore } from "@/stores/run";
import { useRunActivityView } from "@/hooks/useRunActivityView";

function statusColor(status: string) {
  if (status === "succeeded") return "success";
  if (status === "failed") return "error";
  if (status === "cancelled") return "warning";
  if (status === "running") return "processing";
  return "default";
}

function formatDuration(ms?: number) {
  if (ms == null) return "—";
  if (ms < 1000) return `${ms} ms`;
  return `${(ms / 1000).toFixed(1)} s`;
}

const emptyView = (runId: string): RunViewState => ({
  runId,
  seq: 0,
  status: "running",
  statusLabel: "",
  replyText: "",
  turns: [],
});

export default function StageTaskDetailPage() {
  const { id: projectId = "", taskId = "" } = useParams();
  const location = useLocation();
  const navigate = useNavigate();
  const pathKind = stageKindFromPath(location.pathname);
  const kind = pathKind && isStageKind(pathKind) ? pathKind : "plan";
  const stageTitle = stageTitles[kind] || kind;
  const currentRun = useRunStore((s) => s.current);
  const setCurrent = useRunStore((s) => s.setCurrent);
  const pollTimerRef = useRef<ReturnType<typeof setInterval> | null>(null);
  const running =
    currentRun?.status === "running" ||
    currentRun?.status === "queued" ||
    currentRun?.status === "pending";
  const {
    state: viewState,
    loading: viewLoading,
    disconnected: viewDisconnected,
    reload: reloadView,
  } = useRunActivityView(projectId, taskId, {
    live: running,
    mode: "detail",
  });

  async function refreshRun() {
    const run = await runsApi.getRun(projectId, taskId);
    if (run.kind !== kind) {
      if (isStageKind(run.kind)) {
        navigate(`/projects/${projectId}/${run.kind}/${taskId}`, {
          replace: true,
        });
      } else {
        navigate(`/projects/${projectId}`, { replace: true });
      }
      return run;
    }
    setCurrent(run);
    return run;
  }

  function stopPolling() {
    if (pollTimerRef.current) {
      clearInterval(pollTimerRef.current);
      pollTimerRef.current = null;
    }
  }

  useEffect(() => {
    let cancelled = false;
    async function load() {
      const run = await refreshRun();
      if (cancelled) return;
      await reloadView();
      if (cancelled) return;
      const active =
        run.status === "running" ||
        run.status === "queued" ||
        run.status === "pending";
      if (active) {
        pollTimerRef.current = setInterval(async () => {
          const updated = await runsApi.getRun(projectId, taskId);
          setCurrent(updated);
          if (["running", "queued", "pending"].includes(updated.status)) {
            await reloadView();
          }
          if (
            !["running", "queued", "pending"].includes(updated.status)
          ) {
            stopPolling();
            await reloadView();
          }
        }, runStatusPollIntervalMs());
      }
    }
    load();
    return () => {
      cancelled = true;
      stopPolling();
    };
  }, [projectId, taskId, setCurrent, reloadView]);

  async function cancel() {
    await runsApi.cancelRun(projectId, taskId);
    await refreshRun();
    await reloadView();
  }

  const panelState = viewState ?? emptyView(taskId);
  const { result } = panelState;
  const terminal = currentRun && !running;
  const resultStatus = useMemo(() => {
    if (!terminal) return null;
    if (currentRun.status === "succeeded") return "success" as const;
    if (currentRun.status === "cancelled") return "warning" as const;
    return "error" as const;
  }, [terminal, currentRun?.status]);
  const resultTitle = useMemo(() => {
    if (!currentRun) return "";
    if (currentRun.status === "succeeded") return "任务执行成功";
    if (currentRun.status === "cancelled") return "任务已取消";
    if (currentRun.status === "failed") return "任务执行失败";
    return runStatusLabels[currentRun.status] || currentRun.status;
  }, [currentRun]);
  const runningExtra = useMemo(() => {
    if (!running) return null;
    return (
      <Space size="small">
        <Spin size="small" />
        <Typography.Text type="secondary">
          {panelState.statusLabel || "正在准备运行过程…"}
        </Typography.Text>
      </Space>
    );
  }, [panelState.statusLabel, running]);

  return (
    <Space direction="vertical" size="large" style={{ width: "100%" }}>
      <Breadcrumb
        items={[
          { title: <Link to={`/projects/${projectId}`}>概览</Link> },
          {
            title: (
              <Link to={`/projects/${projectId}/${kind}`}>{stageTitle}</Link>
            ),
          },
          { title: "任务详情" },
        ]}
      />
      <Flex justify="space-between" align="flex-start" gap="middle" wrap="wrap">
        <div>
          <Typography.Title level={3} style={{ marginTop: 0, marginBottom: 8 }}>
            {currentRun?.title || taskId}
          </Typography.Title>
          {currentRun && (
            <Tag color={statusColor(currentRun.status)}>
              {runStatusLabels[currentRun.status] || currentRun.status}
            </Tag>
          )}
        </div>
        <Space wrap>
          {(currentRun?.status === "running" ||
            currentRun?.status === "queued") && (
            <Button danger onClick={cancel}>
              取消
            </Button>
          )}
        </Space>
      </Flex>
      {currentRun && (
        <Descriptions size="small" column={{ xs: 1, sm: 2 }} colon>
          {currentRun.sandbox_path && (
            <Descriptions.Item label="沙箱">
              {currentRun.sandbox_path}
            </Descriptions.Item>
          )}
          {currentRun.started_at && (
            <Descriptions.Item label="开始时间">
              {currentRun.started_at}
            </Descriptions.Item>
          )}
          {currentRun.finished_at && (
            <Descriptions.Item label="结束时间">
              {currentRun.finished_at}
            </Descriptions.Item>
          )}
          {currentRun.error_message && (
            <Descriptions.Item label="错误">
              <Typography.Text type="danger">
                {currentRun.error_message}
              </Typography.Text>
            </Descriptions.Item>
          )}
        </Descriptions>
      )}
      <Card
        title="运行过程"
        extra={runningExtra}
        styles={{ body: { maxHeight: "65vh", overflow: "auto" } }}
      >
        {viewDisconnected && running ? (
          <Alert
            type="warning"
            showIcon
            style={{ marginBottom: 12 }}
            message="实时连接暂时中断，页面会继续轮询刷新运行状态。"
          />
        ) : null}
        {viewLoading && !viewState ? (
          <Spin />
        ) : !viewState && !running ? (
          <Empty description="暂无活动" />
        ) : (
          <RunActivityPanel
            state={panelState}
            running={running}
            projectId={projectId}
          />
        )}
        {result && (result.numTurns != null || result.durationMs != null) && (
          <Flex gap="large" style={{ marginTop: 24 }}>
            {result.numTurns != null && (
              <Statistic title="轮次" value={result.numTurns} />
            )}
            {result.durationMs != null && (
              <Statistic
                title="耗时"
                value={formatDuration(result.durationMs)}
              />
            )}
          </Flex>
        )}
        {resultStatus && (
          <Result
            style={{ marginTop: 16 }}
            status={resultStatus}
            title={resultTitle}
            subTitle={
              result?.error || result?.output || currentRun?.error_message
            }
          />
        )}
      </Card>
    </Space>
  );
}
