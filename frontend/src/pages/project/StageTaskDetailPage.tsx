import { useEffect, useRef, useState } from 'react'
import { Link, useLocation, useNavigate, useParams } from 'react-router-dom'
import { Alert, Breadcrumb, Button, Empty, Space, Tag, Typography } from 'antd'
import { useRunStore } from '@/stores/run'
import { subscribeRunStream } from '@/api/stream'
import * as runsApi from '@/api/runs'
import type { RunEvent } from '@/api/runs'
import { runStatusLabels, stageTitles } from '@/locales/zh-CN'
import { isStageKind, stageKindFromPath } from '@/utils/stage'

export default function StageTaskDetailPage() {
  const { id: projectId = '', taskId = '' } = useParams()
  const location = useLocation()
  const navigate = useNavigate()
  const pathKind = stageKindFromPath(location.pathname)
  const kind = pathKind && isStageKind(pathKind) ? pathKind : 'plan'
  const stageTitle = stageTitles[kind] || kind

  const currentRun = useRunStore((s) => s.current)
  const streamEvents = useRunStore((s) => s.streamEvents)
  const setCurrent = useRunStore((s) => s.setCurrent)
  const appendStream = useRunStore((s) => s.appendStream)

  const [events, setEvents] = useState<RunEvent[]>([])
  const [mergeError, setMergeError] = useState('')
  const [conflicts, setConflicts] = useState<string[]>([])
  const [acting, setActing] = useState(false)

  const unsubscribeRef = useRef<(() => void) | null>(null)
  const pollTimerRef = useRef<ReturnType<typeof setInterval> | null>(null)

  async function loadEvents(afterId?: string) {
    const res = await runsApi.listRunEvents(projectId, taskId, afterId)
    if (res.events.length) setEvents((prev) => [...prev, ...res.events])
    return res.events.at(-1)?.id
  }

  async function refreshRun() {
    const run = await runsApi.getRun(projectId, taskId)
    if (run.kind !== kind) {
      if (isStageKind(run.kind)) {
        navigate(`/projects/${projectId}/${run.kind}/${taskId}`, { replace: true })
      } else {
        navigate(`/projects/${projectId}`, { replace: true })
      }
      return run
    }
    setCurrent(run)
    return run
  }

  useEffect(() => {
    async function load() {
      const run = await refreshRun()
      setEvents([])
      await loadEvents()

      const active = run.status === 'running' || run.status === 'queued' || run.status === 'pending'
      if (active) {
        unsubscribeRef.current = subscribeRunStream(projectId, taskId, (msg) => {
          appendStream(msg)
        })
        pollTimerRef.current = setInterval(async () => {
          await loadEvents(events.at(-1)?.id)
          const updated = await runsApi.getRun(projectId, taskId)
          setCurrent(updated)
          if (!['running', 'queued', 'pending'].includes(updated.status)) {
            if (pollTimerRef.current) clearInterval(pollTimerRef.current)
            pollTimerRef.current = null
          }
        }, 3000)
      }
    }
    load()
    return () => {
      unsubscribeRef.current?.()
      if (pollTimerRef.current) clearInterval(pollTimerRef.current)
    }
  }, [projectId, taskId, setCurrent, appendStream])

  async function cancel() {
    await runsApi.cancelRun(projectId, taskId)
    await refreshRun()
  }

  async function merge() {
    setActing(true)
    setMergeError('')
    setConflicts([])
    try {
      const run = await runsApi.mergeRun(projectId, taskId)
      setCurrent(run)
      setConflicts([])
    } catch (e) {
      const err = e as Error & { conflicts?: string[] }
      setMergeError(err.message || '合并失败')
      if (err.conflicts?.length) setConflicts(err.conflicts)
    } finally {
      setActing(false)
    }
  }

  async function discard() {
    setActing(true)
    try {
      const run = await runsApi.discardRun(projectId, taskId)
      setCurrent(run)
    } finally {
      setActing(false)
    }
  }

  const eventText = events.length
    ? events.map((e) => e.event_type + ': ' + (e.payload || '')).join('\n---\n')
    : streamEvents.join('\n---\n')

  const canMerge = currentRun?.status === 'succeeded' && currentRun.merge_status === 'pending'

  return (
    <div className="stack">
      <Breadcrumb
        items={[
          { title: <Link to={`/projects/${projectId}`}>概览</Link> },
          { title: <Link to={`/projects/${projectId}/${kind}`}>{stageTitle}</Link> },
          { title: '任务详情' },
        ]}
      />
      <div className="flex-between">
        <div>
          <h2>{currentRun?.title || taskId}</h2>
          {currentRun && (
            <Space wrap>
              <Tag color={currentRun.status === 'succeeded' ? 'success' : currentRun.status === 'failed' || currentRun.status === 'cancelled' ? 'error' : currentRun.status === 'running' ? 'processing' : 'default'}>
                {runStatusLabels[currentRun.status] || currentRun.status}
              </Tag>
              {currentRun.merge_status && (
                <Tag>{currentRun.merge_status === 'pending' ? '待合并' : currentRun.merge_status === 'merged' ? '已合并' : '已放弃'}</Tag>
              )}
            </Space>
          )}
          {currentRun?.sandbox_path && (
            <Typography.Paragraph type="secondary" style={{ fontSize: 12, marginBottom: 0 }}>
              沙箱：{currentRun.sandbox_path}
            </Typography.Paragraph>
          )}
          {currentRun?.error_message && (
            <p style={{ color: 'var(--matrix-color-red-500)' }}>{currentRun.error_message}</p>
          )}
        </div>
        <Space>
          {canMerge && (
            <>
              <Button type="primary" loading={acting} onClick={merge}>合并到主仓库</Button>
              <Button loading={acting} onClick={discard}>放弃</Button>
            </>
          )}
          {(currentRun?.status === 'running' || currentRun?.status === 'queued') && (
            <Button danger onClick={cancel}>取消</Button>
          )}
        </Space>
      </div>
      {mergeError && (
        <Alert
          type="error"
          showIcon
          message={mergeError}
          description={conflicts.length ? `冲突文件：${conflicts.join(', ')}` : undefined}
        />
      )}
      <div className="panel" style={{ maxHeight: '60vh', overflow: 'auto' }}>
        <Typography.Title level={5} style={{ marginTop: 0 }}>输出</Typography.Title>
        {eventText ? (
          <Typography.Paragraph>
            <pre style={{ margin: 0, whiteSpace: 'pre-wrap', wordBreak: 'break-word', fontSize: 12 }}>{eventText}</pre>
          </Typography.Paragraph>
        ) : (
          <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="等待输出…" />
        )}
      </div>
    </div>
  )
}
