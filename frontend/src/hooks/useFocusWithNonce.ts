import { useEffect } from "react";
import { useFocusStore } from "@/stores/useFocusStore";
import type { PanelId } from "@/types/panel";

// useFocusWithNonce: wraps the focusStore subscription + unmount cleanup.
// Returns the current panel's focus entry ({ id, nonce } | undefined).
// On View unmount it auto-clears clearFocus(panelId), avoiding a stale focusId
// triggering an unwanted re-position when switching away and back to remount.
// Each View's positioning logic (scrollIntoView / setExpandedId /
// setWindowCenter, etc.) lives in its own useEffect, with focus?.nonce added
// as a dependency so "clicking the same entry again" also triggers it.
export function useFocusWithNonce(panelId: PanelId) {
  const focus = useFocusStore((s) => s.focusMap[panelId]);
  const clear = useFocusStore((s) => s.clearFocus);
  useEffect(() => {
    return () => clear(panelId);
  }, [panelId, clear]);
  return focus;
}
