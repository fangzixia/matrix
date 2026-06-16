export type StreamHandler = (data: unknown) => void

export function subscribeRunStream(projectId: string, runId: string, onMessage: StreamHandler): () => void {
  const url = `/api/projects/${projectId}/runs/${runId}/stream`
  const es = new EventSource(url, { withCredentials: true })

  es.addEventListener('agent:stream', (ev) => {
    try {
      onMessage(JSON.parse((ev as MessageEvent).data))
    } catch {
      onMessage((ev as MessageEvent).data)
    }
  })

  es.onerror = () => {
    es.close()
  }

  return () => es.close()
}
