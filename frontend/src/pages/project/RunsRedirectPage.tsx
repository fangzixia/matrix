import { useEffect, useState } from 'react'
import { Navigate, useParams } from 'react-router-dom'
import { Spin } from 'antd'
import * as runsApi from '@/api/runs'
import { isStageKind } from '@/utils/stage'

/** 兼容旧 /runs/:runId 链接，重定向到对应阶段任务页。 */
export default function RunsRedirectPage() {
  const { id = '', runId = '' } = useParams()
  const [target, setTarget] = useState<string | null>(null)

  useEffect(() => {
    let cancelled = false
    runsApi.getRun(id, runId).then((run) => {
      if (cancelled) return
      if (isStageKind(run.kind)) {
        setTarget(`/projects/${id}/${run.kind}/${runId}`)
      } else {
        setTarget(`/projects/${id}`)
      }
    }).catch(() => {
      if (!cancelled) setTarget(`/projects/${id}`)
    })
    return () => { cancelled = true }
  }, [id, runId])

  if (!target) {
    return <Spin style={{ display: 'block', margin: '48px auto' }} />
  }
  return <Navigate to={target} replace />
}
