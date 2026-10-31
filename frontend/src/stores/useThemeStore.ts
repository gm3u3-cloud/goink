import { create } from "zustand";
import { persist, createJSONStorage } from "zustand/middleware";

const ATTR = "data-theme";

const THEMES = ["light", "dark"] as const;
export type Theme = (typeof THEMES)[number];

const NEXT: Record<Theme, Theme> = { light: "dark", dark: "light" };

function isTheme(s: unknown): s is Theme {
  return typeof s === "string" && THEMES.includes(s as Theme);
}

function applyTheme(t: Theme) {
  document.documentElement.setAttribute(ATTR, t);
}

interface ThemeState {
  theme: Theme;
  setTheme: (t: Theme) => void;
  toggle: () => void;
}

// 7.1: useTheme migrated to a zustand store + persist. The old
// MutationObserver cross-component sync hack is removed (a store as a single
// source of truth syncs across components by nature). The old hand-written
// localStorage read/write is now handled by the persist middleware.
// Behavior change: first launch no longer follows the system theme (the old
// localStorage.getItem("theme") === null fallback is removed); the default is
// light, and once the user picks a theme it persists. This behavior change is
// accepted in exchange for a simpler store.
export const useThemeStore = create<ThemeState>()(
  persist(
    (set, get) => ({
      theme: "light",
      setTheme: (t) => {
        applyTheme(t);
        set({ theme: t });
      },
      toggle: () => {
        const next = NEXT[get().theme];
        applyTheme(next);
        set({ theme: next });
      },
    }),
    {
      name: "goink-theme",
      storage: createJSONStorage(() => localStorage),
      partialize: (s) => ({ theme: s.theme }),
      onRehydrateStorage: () => (state) => {
        if (state && isTheme(state.theme)) {
          applyTheme(state.theme);
        }
      },
    },
  ),
);
