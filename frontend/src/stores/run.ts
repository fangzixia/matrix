import { create } from "zustand";
import * as runsApi from "@/api/runs";
import type { Run } from "@/api/runs";
import type { StreamMessage } from "@/types/runStream";
import { isStageKind } from "@/utils/stage";

interface RunState {
  runs: Run[];
  current: Run | null;
  loading: boolean;
  streamMessages: StreamMessage[];
  fetchRuns: (projectId: string, kind?: string) => Promise<void>;
  setCurrent: (run: Run | null) => void;
  appendStream: (data: unknown) => void;
}

let fetchSeq = 0;

function asStreamMessage(data: unknown): StreamMessage | null {
  if (!data || typeof data !== "object") return null;
  return data as StreamMessage;
}

export const useRunStore = create<RunState>((set, get) => ({
  runs: [],
  current: null,
  loading: false,
  streamMessages: [],
  fetchRuns: async (projectId, kind?) => {
    const seq = ++fetchSeq;
    set({ loading: true, runs: [] });
    try {
      const res = await runsApi.listRuns(projectId, kind);
      if (seq !== fetchSeq) return;
      const runs =
        kind && isStageKind(kind)
          ? res.runs.filter((r) => r.kind === kind)
          : res.runs;
      set({ runs });
    } finally {
      if (seq === fetchSeq) set({ loading: false });
    }
  },
  setCurrent: (run) => set({ current: run, streamMessages: [] }),
  appendStream: (data) => {
    const msg = asStreamMessage(data);
    if (!msg) return;
    set({ streamMessages: [...get().streamMessages, msg] });
  },
}));
