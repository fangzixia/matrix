import { create } from "zustand";
import * as authApi from "@/api/auth";
import type { User } from "@/api/auth";

interface AuthState {
  user: User | null;
  loading: boolean;
  initialized: boolean;
  isRoot: () => boolean;
  fetchMe: () => Promise<void>;
  login: (username: string, password: string) => Promise<void>;
  logout: () => Promise<void>;
  setUser: (user: User | null) => void;
}

export const useAuthStore = create<AuthState>((set, get) => ({
  user: null,
  loading: false,
  initialized: false,
  isRoot: () => !!get().user?.is_root,
  setUser: (user) => set({ user }),
  fetchMe: async () => {
    set({ loading: true });
    try {
      const user = await authApi.me();
      set({ user });
    } catch {
      set({ user: null });
    } finally {
      set({ loading: false, initialized: true });
    }
  },
  login: async (username, password) => {
    const user = await authApi.login(username, password);
    set({ user });
  },
  logout: async () => {
    await authApi.logout();
    set({ user: null });
  },
}));
