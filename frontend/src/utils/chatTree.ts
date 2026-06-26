/**
 * Chat 会话消息树工具（parent_id 分支）。
 */
import type { ChatMessageNode } from "@/api/chat";
import type { AiMessage } from "@/components/ai/MatrixAiChat";

function strOrNull(v: string | null | undefined): string | null {
  const s = (v ?? "").trim();
  return s || null;
}

/** 从根到指定节点（含）还原线性列表，用于重发前的上下文展示。 */
export function branchThroughNode(
  nodes: ChatMessageNode[],
  nodeId: string | null | undefined,
): AiMessage[] {
  const id = (nodeId ?? "").trim();
  if (!id) return [];
  const byId = new Map(nodes.map((n) => [n.id, n]));
  if (!byId.has(id)) return [];

  const chain: ChatMessageNode[] = [];
  const seen = new Set<string>();
  let cur: string | null = id;
  while (cur) {
    if (seen.has(cur)) break;
    seen.add(cur);
    const node = byId.get(cur);
    if (!node) break;
    chain.push(node);
    cur = strOrNull(node.parent_id);
  }
  chain.reverse();

  return chain.map((n) => ({
    key: n.id,
    role: n.role === "user" ? "user" : "ai",
    content: n.content,
    attachments: n.attachments,
    runId: n.run_id,
    parentId: n.parent_id ?? undefined,
  }));
}

/** 从 active leaf 回溯，还原 UI 线性消息列表（根→叶）。 */
export function branchToItems(
  nodes: ChatMessageNode[],
  activeLeafId: string | null | undefined,
): AiMessage[] {
  if (!nodes.length) return [];
  const leafId = (activeLeafId ?? "").trim() || nodes[nodes.length - 1]?.id;
  return branchThroughNode(nodes, leafId);
}

/** 普通发送：挂到当前 active leaf。 */
export function parentIdForSend(
  activeLeafId: string | null | undefined,
): string | null {
  return strOrNull(activeLeafId);
}

/** 重发用户消息：挂到该消息的 parent（开新分支）。 */
export function parentIdForResend(userMsg: {
  parentId?: string | null;
}): string | null {
  return strOrNull(userMsg.parentId);
}

/** 从 active 分支取会话标题。 */
export function sessionTitleFromBranch(
  nodes: ChatMessageNode[],
  activeLeafId: string | null | undefined,
): string {
  const items = branchToItems(nodes, activeLeafId);
  const first = items.find((m) => m.role === "user" && m.content.trim());
  if (first?.content) return first.content.trim().slice(0, 80);
  return "新对话";
}

export function hasBranchMessages(
  nodes: ChatMessageNode[],
  activeLeafId?: string | null,
): boolean {
  const items = branchToItems(nodes, activeLeafId);
  return items.some(
    (m) => m.content.trim() || (m.attachments?.length ?? 0) > 0,
  );
}

/** 会话是否有实质内容（含尚未加载消息树的摘要会话）。 */
export function sessionHasContent(
  nodes: ChatMessageNode[],
  activeLeafId?: string | null,
): boolean {
  if ((activeLeafId ?? "").trim()) return true;
  return hasBranchMessages(nodes, activeLeafId);
}

/** 将 API 会话转为本地状态。 */
export function sessionFromApi(
  session: {
    id: string;
    title?: string;
    model_id?: string;
    active_leaf_id?: string | null;
    nodes?: ChatMessageNode[];
    updated_at?: string;
  },
  defaultModelId?: string,
) {
  const nodes = session.nodes ?? [];
  const activeLeafId = session.active_leaf_id ?? null;
  return {
    id: session.id,
    title:
      session.title ||
      sessionTitleFromBranch(nodes, activeLeafId) ||
      "对话",
    modelId: session.model_id || defaultModelId,
    nodes,
    activeLeafId,
    updatedAt: session.updated_at,
  };
}
