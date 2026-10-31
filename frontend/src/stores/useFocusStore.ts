import { create } from "zustand";
import type { PanelId } from "@/types/panel";

interface FocusEntry {
  id: number;
  nonce: number;
  type?: "arc" | "node"; // 4b: storyarc distinguishes arc/node jumps (other domains don't pass it, undefined)
}

interface FocusState {
  focusMap: Partial<Record<PanelId, FocusEntry>>;
  focusEntity: (panelId: PanelId, id: number, type?: "arc" | "node") => void;
  clearFocus: (panelId: PanelId) => void;
}

// focusMap is replaced wholesale (not a single-key merge), equivalent to the
// setFocusMap semantics from 2.6: during search navigation, clear the other
// panels' stale focusId, keeping only the current panel's.
// focusEntity writes with a nonce so "clicking the same entry again" also
// triggers the useEffect (id unchanged, nonce changed).
// clearFocus clears itself on View unmount (cleanup registered by the
// useFocusWithNonce hook), avoiding a stale focusId triggering an unwanted
// re-position when switching away and back to remount.
export const useFocusStore = create<FocusState>((set) => ({
  focusMap: {},
  focusEntity: (panelId, id, type) =>
    set({
      focusMap: {
        [panelId]: { id, nonce: Date.now(), type },
      } as Partial<Record<PanelId, FocusEntry>>,
    }),
  clearFocus: (panelId) =>
    set((s) => {
      const next = { ...s.focusMap };
      delete next[panelId];
      return { focusMap: next };
    }),
}));
