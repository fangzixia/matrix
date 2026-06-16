export interface ApiError {
  error: string
  message: string
}

export class HttpError extends Error {
  status: number
  code: string

  constructor(status: number, code: string, message: string) {
    super(message)
    this.status = status
    this.code = code
  }
}

export async function api<T>(path: string, init: RequestInit = {}): Promise<T> {
  const headers = new Headers(init.headers)
  if (init.body && !headers.has('Content-Type')) {
    headers.set('Content-Type', 'application/json')
  }
  const res = await fetch(path, {
    ...init,
    headers,
    credentials: 'include',
  })
  if (!res.ok) {
    let code = 'error'
    let message = res.statusText
    try {
      const body = (await res.json()) as ApiError
      code = body.error || code
      message = body.message || message
    } catch {
      /* ignore */
    }
    throw new HttpError(res.status, code, message)
  }
  if (res.status === 204) {
    return undefined as T
  }
  const text = await res.text()
  if (!text) return undefined as T
  return JSON.parse(text) as T
}
