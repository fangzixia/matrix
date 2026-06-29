/**
 * HTTP 客户端：封装 fetch，携带 Session Cookie，统一解析 JSON 错误。
 */

/** 后端 JSON 错误响应体 */
export interface ApiError {
  error: string;
  message: string;
  code?: string;
  details?: unknown;
}

/** 带 status/code 的 HTTP 异常，供页面捕获并展示 message */
export class HttpError extends Error {
  status: number;
  code: string;
  details?: unknown;
  constructor(status: number, code: string, message: string, details?: unknown) {
    super(message);
    this.status = status;
    this.code = code;
    this.details = details;
  }
}

/**
 * api 发起 REST 请求；204/空响应返回 undefined。
 * @param path 相对路径（如 /api/projects）
 */
export async function api<T>(path: string, init: RequestInit = {}): Promise<T> {
  const headers = new Headers(init.headers);
  if (init.body && !headers.has("Content-Type")) {
    headers.set("Content-Type", "application/json");
  }
  const res = await fetch(path, {
    ...init,
    headers,
    credentials: "include",
  });
  if (!res.ok) {
    let code = "error";
    let message = res.statusText;
    let details: unknown;
    try {
      const body = (await res.json()) as ApiError;
      code = body.code || body.error || code;
      message = body.message || message;
      details = body.details;
    } catch {
      /* ignore */
    }
    throw new HttpError(res.status, code, message, details);
  }
  if (res.status === 204) {
    return undefined as T;
  }
  const text = await res.text();
  if (!text) return undefined as T;
  return JSON.parse(text) as T;
}
