import { create } from "zustand";
import { persist, createJSONStorage } from "zustand/middleware";

export const FONT_SCALE_MIN = 75;
export const FONT_SCALE_MAX = 150;
export const FONT_SCALE_STEP = 5;
export const FONT_SCALE_DEFAULT = 100;

// 9.1: font scale is a UI-only preference, persisted to localStorage like
// the theme (see useThemeStore). The app_config DB row is reserved for
// important app-level settings (see internal/config/store.go). Scaling the
// root font-size scales every rem-based Tailwind class in the UI
// proportionally, so a single value covers all interface text.
function clampScale(v: number): number {
  const stepped = Math.round(v / FONT_SCALE_STEP) * FONT_SCALE_STEP;
  return Math.min(FONT_SCALE_MAX, Math.max(FONT_SCALE_MIN, stepped));
}

function isScale(v: unknown): v is number {
  return typeof v === "number" && Number.isFinite(v);
}

function applyScale(scale: number) {
  document.documentElement.style.fontSize = `${clampScale(scale)}%`;
}

interface FontScaleState {
  scale: number;
  setScale: (s: number) => void;
  reset: () => void;
}

export const useFontScaleStore = create<FontScaleState>()(
  persist(
    (set) => ({
      scale: FONT_SCALE_DEFAULT,
      setScale: (s) => {
        const next = clampScale(s);
        applyScale(next);
        set({ scale: next });
      },
      reset: () => {
        applyScale(FONT_SCALE_DEFAULT);
        set({ scale: FONT_SCALE_DEFAULT });
      },
    }),
    {
      name: "goink-font-scale",
      storage: createJSONStorage(() => localStorage),
      partialize: (s) => ({ scale: s.scale }),
      onRehydrateStorage: () => (state) => {
        if (state && isScale(state.scale)) {
          applyScale(state.scale);
        }
      },
    },
  ),
);
