import { create } from "zustand";
import * as runsApi from "@/api/runs";
import type { Run } from "@/api/runs";

import type { RunKind, StageKind } from "@/types/runKind";

interface RunState {
  runs: Run[];
  current: Run | null;
  loading: boolean;
  fetchRuns: (projectId: string, kind?: RunKind | StageKind) => Promise<void>;
  setCurrent: (run: Run | null) => void;
}

let fetchSeq = 0;

export const useRunStore = create<RunState>((set) => ({
  runs: [],
  current: null,
  loading: false,
  fetchRuns: async (projectId, kind?) => {
    const seq = ++fetchSeq;
    set({ loading: true, runs: [] });
    try {
      const res = await runsApi.listRuns(projectId, kind);
      if (seq !== fetchSeq) return;
      set({ runs: res.runs });
    } finally {
      if (seq === fetchSeq) set({ loading: false });
    }
  },
  setCurrent: (run) => set({ current: run }),
}));
