import { useEffect, useRef, useState } from 'react'
import { useParams } from 'react-router-dom'
import { Button, Empty, Steps, Tag, Typography } from 'antd'
import { useRunStore } from '@/stores/run'
import { subscribeRunStream } from '@/api/stream'
import * as runsApi from '@/api/runs'
import type { RunStep, RunEvent } from '@/api/runs'
import { runKindLabels, runStatusLabels } from '@/locales/zh-CN'

function stepStatus(status: string): 'wait' | 'process' | 'finish' | 'error' {
  if (status === 'succeeded') return 'finish'
  if (status === 'failed' || status === 'cancelled') return 'error'
  if (status === 'running' || status === 'queued') return 'process'
  return 'wait'
}

export default function RunDetailPage() {
  const { id: projectId = '', runId = '' } = useParams()
  const currentRun = useRunStore((s) => s.current)
  const streamEvents = useRunStore((s) => s.streamEvents)
  const setCurrent = useRunStore((s) => s.setCurrent)
  const appendStream = useRunStore((s) => s.appendStream)
  const [steps, setSteps] = useState<RunStep[]>([])
  const [events, setEvents] = useState<RunEvent[]>([])
  const unsubscribeRef = useRef<(() => void) | null>(null)
  const pollTimerRef = useRef<ReturnType<typeof setInterval> | null>(null)

  async function loadEvents(afterId?: string) {
    const res = await runsApi.listRunEvents(projectId, runId, afterId)
    if (res.events.length) setEvents((prev) => [...prev, ...res.events])
    return res.events.at(-1)?.id
  }

  useEffect(() => {
    async function load() {
      const run = await runsApi.getRun(projectId, runId)
      setCurrent(run)
      const stepsRes = await runsApi.listRunSteps(projectId, runId)
      setSteps(stepsRes.steps)
      setEvents([])
      await loadEvents()

      const active = run.status === 'running' || run.status === 'queued' || run.status === 'pending'
      if (active) {
        unsubscribeRef.current = subscribeRunStream(projectId, runId, (msg) => {
          appendStream(msg)
        })
        pollTimerRef.current = setInterval(async () => {
          const lastId = events.at(-1)?.id
          await loadEvents(lastId)
          const updated = await runsApi.getRun(projectId, runId)
          setCurrent(updated)
          if (!['running', 'queued', 'pending'].includes(updated.status)) {
            if (pollTimerRef.current) clearInterval(pollTimerRef.current)
            pollTimerRef.current = null
            const stepsRes2 = await runsApi.listRunSteps(projectId, runId)
            setSteps(stepsRes2.steps)
          }
        }, 3000)
      }
    }
    load()
    return () => {
      unsubscribeRef.current?.()
      if (pollTimerRef.current) clearInterval(pollTimerRef.current)
    }
  }, [projectId, runId, setCurrent, appendStream])

  async function cancel() {
    await runsApi.cancelRun(projectId, runId)
    const run = await runsApi.getRun(projectId, runId)
    setCurrent(run)
  }

  const eventText = events.length
    ? events.map((e) => e.event_type + ': ' + (e.payload || '')).join('\n---\n')
    : streamEvents.join('\n---\n')

  return (
    <div className="stack">
      <div className="flex-between">
        <div>
          <h2>{currentRun?.title || runId}</h2>
          {currentRun && (
            <Tag color={currentRun.status === 'succeeded' ? 'success' : currentRun.status === 'failed' || currentRun.status === 'cancelled' ? 'error' : currentRun.status === 'running' ? 'processing' : 'default'}>
              {runStatusLabels[currentRun.status] || currentRun.status}
            </Tag>
          )}
          {currentRun?.error_message && (
            <p style={{ color: 'var(--matrix-color-red-500)' }}>{currentRun.error_message}</p>
          )}
        </div>
        {(currentRun?.status === 'running' || currentRun?.status === 'queued') && (
          <Button danger onClick={cancel}>取消</Button>
        )}
      </div>
      {steps.length > 0 && (
        <div className="panel">
          <Typography.Title level={5} style={{ marginTop: 0 }}>流水线步骤</Typography.Title>
          <Steps
            direction="vertical"
            size="small"
            items={steps.map((s) => ({
              title: runKindLabels[s.kind] || s.kind,
              status: stepStatus(s.status),
              description: (
                <>
                  <Tag>{runStatusLabels[s.status] || s.status}</Tag>
                  {s.output_summary && (
                    <Typography.Paragraph type="secondary" style={{ fontSize: 12, margin: '4px 0 0' }}>
                      {s.output_summary}
                    </Typography.Paragraph>
                  )}
                </>
              ),
            }))}
          />
        </div>
      )}
      <div className="panel" style={{ maxHeight: '50vh', overflow: 'auto' }}>
        <Typography.Title level={5} style={{ marginTop: 0 }}>事件</Typography.Title>
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
