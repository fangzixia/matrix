import type { SystemMCPServer } from '@/api/system'

/** 解析 MCP JSON，兼容 Cursor `mcp.json`（含 mcpServers 包装）与裸服务对象。 */
export function parseMcpServersJson(text: string): Record<string, SystemMCPServer> {
  const trimmed = text.trim()
  if (!trimmed) {
    return {}
  }
  let raw: unknown
  try {
    raw = JSON.parse(trimmed)
  } catch {
    throw new Error('MCP 配置 JSON 格式无效')
  }
  if (!raw || typeof raw !== 'object' || Array.isArray(raw)) {
    throw new Error('MCP 配置须为 JSON 对象')
  }
  const obj = raw as Record<string, unknown>
  if (obj.mcpServers && typeof obj.mcpServers === 'object' && !Array.isArray(obj.mcpServers)) {
    return normalizeMcpServers(obj.mcpServers as Record<string, unknown>)
  }
  if (obj.mcp_servers && typeof obj.mcp_servers === 'object' && !Array.isArray(obj.mcp_servers)) {
    return normalizeMcpServers(obj.mcp_servers as Record<string, unknown>)
  }
  return normalizeMcpServers(obj)
}

function normalizeMcpServers(raw: Record<string, unknown>): Record<string, SystemMCPServer> {
  const out: Record<string, SystemMCPServer> = {}
  for (const [name, val] of Object.entries(raw)) {
    if (!val || typeof val !== 'object' || Array.isArray(val)) {
      continue
    }
    const s = val as Record<string, unknown>
    const entry: SystemMCPServer = {}
    if (typeof s.command === 'string') entry.command = s.command
    if (Array.isArray(s.args)) entry.args = s.args.filter((a): a is string => typeof a === 'string')
    if (typeof s.url === 'string') entry.url = s.url
    if (typeof s.disabled === 'boolean') entry.disabled = s.disabled
    if (s.headers && typeof s.headers === 'object' && !Array.isArray(s.headers)) {
      entry.headers = Object.fromEntries(
        Object.entries(s.headers as Record<string, unknown>).filter(([, v]) => typeof v === 'string'),
      ) as Record<string, string>
    }
    if (s.env && typeof s.env === 'object' && !Array.isArray(s.env)) {
      entry.env = Object.fromEntries(
        Object.entries(s.env as Record<string, unknown>).filter(([, v]) => typeof v === 'string'),
      ) as Record<string, string>
    }
    out[name] = entry
  }
  return out
}
