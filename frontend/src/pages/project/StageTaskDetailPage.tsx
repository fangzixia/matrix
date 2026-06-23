import { useEffect, useMemo, useRef, useState } from "react";
import { Link, useLocation, useNavigate, useParams } from "react-router-dom";
import {
  Alert,
  Breadcrumb,
  Button,
  Card,
  Descriptions,
  Flex,
  Progress,
  Result,
  Space,
  Spin,
  Statistic,
  Steps,
  Tag,
  Typography,
} from "antd";
import RunActivityPanel from "@/components/ai/RunActivityPanel";
import { useRunStore } from "@/stores/run";
import { subscribeRunStream } from "@/api/stream";
import * as runsApi from "@/api/runs";
import type { RunEvent, RunStep } from "@/api/runs";
import { runStatusLabels, stageTitles } from "@/locales/zh-CN";
import { buildRunActivityState } from "@/utils/runActivityParser";
import { canMergeRun, isStageKind, stageKindFromPath } from "@/utils/stage";

function statusColor(status: string) {
  if (status === "succeeded") return "success";
  if (status === "failed" || status === "cancelled") return "error";
  if (status === "running") return "processing";
  return "default";
}

function stepStatus(status: string): "wait" | "process" | "finish" | "error" {
  if (status === "running") return "process";
  if (status === "succeeded") return "finish";
  if (status === "failed" || status === "cancelled") return "error";
  return "wait";
}

function formatDuration(ms?: number) {
  if (ms == null) return "—";
  if (ms < 1000) return `${ms} ms`;
  return `${(ms / 1000).toFixed(1)} s`;
}

export default function StageTaskDetailPage() {
  const { id: projectId = "", taskId = "" } = useParams();
  const location = useLocation();
  const navigate = useNavigate();
  const pathKind = stageKindFromPath(location.pathname);
  const kind = pathKind && isStageKind(pathKind) ? pathKind : "plan";
  const stageTitle = stageTitles[kind] || kind;
  const currentRun = useRunStore((s) => s.current);
  const streamMessages = useRunStore((s) => s.streamMessages);
  const setCurrent = useRunStore((s) => s.setCurrent);
  const appendStream = useRunStore((s) => s.appendStream);
  const [events, setEvents] = useState<RunEvent[]>([]);
  const [steps, setSteps] = useState<RunStep[]>([]);
  const [eventsLoading, setEventsLoading] = useState(true);
  const [mergeError, setMergeError] = useState("");
  const [conflicts, setConflicts] = useState<string[]>([]);
  const [acting, setActing] = useState(false);
  const unsubscribeRef = useRef<(() => void) | null>(null);
  const pollTimerRef = useRef<ReturnType<typeof setInterval> | null>(null);
  const lastEventIdRef = useRef<string | undefined>(undefined);
  async function loadEvents(afterId?: string) {
    const res = await runsApi.listRunEvents(projectId, taskId, afterId);
    if (res.events.length) {
      setEvents((prev) => [...prev, ...res.events]);
      lastEventIdRef.current = res.events.at(-1)?.id;
    }
    return res.events.at(-1)?.id;
  }
  async function loadSteps() {
    try {
      const res = await runsApi.listRunSteps(projectId, taskId);
      setSteps(res.steps ?? []);
    } catch {
      setSteps([]);
    }
  }
  async function refreshRun() {
    const run = await runsApi.getRun(projectId, taskId);
    if (run.kind !== kind) {
      if (run.kind === "pipeline" && kind === "build") {
        setCurrent(run);
        return run;
      }
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
  useEffect(() => {
    let cancelled = false;
    async function load() {
      setEventsLoading(true);
      setEvents([]);
      lastEventIdRef.current = undefined;
      const run = await refreshRun();
      if (cancelled) return;
      await Promise.all([loadEvents(), loadSteps()]);
      if (cancelled) return;
      setEventsLoading(false);
      const active =
        run.status === "running" ||
        run.status === "queued" ||
        run.status === "pending";
      if (active) {
        unsubscribeRef.current = subscribeRunStream(
          projectId,
          taskId,
          (msg) => {
            appendStream(msg);
          },
        );
        pollTimerRef.current = setInterval(async () => {
          await loadEvents(lastEventIdRef.current);
          await loadSteps();
          const updated = await runsApi.getRun(projectId, taskId);
          setCurrent(updated);
          if (!["running", "queued", "pending"].includes(updated.status)) {
            if (pollTimerRef.current) clearInterval(pollTimerRef.current);
            pollTimerRef.current = null;
          }
        }, 3000);
      }
    }
    load();
    return () => {
      cancelled = true;
      unsubscribeRef.current?.();
      if (pollTimerRef.current) clearInterval(pollTimerRef.current);
    };
  }, [projectId, taskId, setCurrent, appendStream]);
  async function cancel() {
    await runsApi.cancelRun(projectId, taskId);
    await refreshRun();
  }
  async function merge() {
    setActing(true);
    setMergeError("");
    setConflicts([]);
    try {
      const run = await runsApi.mergeRun(projectId, taskId);
      setCurrent(run);
      setConflicts([]);
    } catch (e) {
      const err = e as Error & { conflicts?: string[] };
      setMergeError(err.message || "合并失败");
      if (err.conflicts?.length) setConflicts(err.conflicts);
    } finally {
      setActing(false);
    }
  }
  async function discard() {
    setActing(true);
    try {
      const run = await runsApi.discardRun(projectId, taskId);
      setCurrent(run);
    } finally {
      setActing(false);
    }
  }
  const running =
    currentRun?.status === "running" ||
    currentRun?.status === "queued" ||
    currentRun?.status === "pending";
  const activityState = useMemo(
    () => buildRunActivityState(events, streamMessages),
    [events, streamMessages],
  );
  const canMerge = currentRun ? canMergeRun(currentRun) : false;
  const { result } = activityState;
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
            <Space wrap>
              <Tag color={statusColor(currentRun.status)}>
                {runStatusLabels[currentRun.status] || currentRun.status}
              </Tag>
              {currentRun.merge_status && (
                <Tag>
                  {currentRun.merge_status === "pending"
                    ? "待合并"
                    : currentRun.merge_status === "merged"
                      ? "已合并"
                      : "已放弃"}
                </Tag>
              )}
            </Space>
          )}
        </div>
        <Space wrap>
          {canMerge && (
            <>
              <Button type="primary" loading={acting} onClick={merge}>
                合并到主仓库
              </Button>
              <Button loading={acting} onClick={discard}>
                放弃
              </Button>
            </>
          )}
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
          {currentRun.run_branch && (
            <Descriptions.Item label="分支">
              {currentRun.run_branch}
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
      {steps.length > 1 && (
        <Steps
          size="small"
          items={steps.map((s) => ({
            title: s.kind || `步骤 ${s.sequence}`,
            description: s.output_summary,
            status: stepStatus(s.status),
          }))}
        />
      )}
      {mergeError && (
        <Alert
          type="error"
          showIcon
          message={mergeError}
          description={
            conflicts.length ? `冲突文件：${conflicts.join(", ")}` : undefined
          }
        />
      )}
      <Card
        title="运行过程"
        extra={
          running ? (
            <Progress
              percent={99}
              status="active"
              showInfo={false}
              style={{ width: 120 }}
            />
          ) : null
        }
        styles={{ body: { maxHeight: "65vh", overflow: "auto" } }}
      >
        <Spin spinning={eventsLoading && !activityState.turns.length}>
          <RunActivityPanel state={activityState} running={running} />
        </Spin>
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
