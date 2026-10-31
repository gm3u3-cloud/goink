import { useCallback, useRef } from "react";
import type { EditorTab } from "@/components/content/types";
import { useEditorTabsStore } from "./useEditorTabsStore";

// useEditorTabs: a thin per-novel selector wrapper over useEditorTabsStore,
// with the same API as the old implementation.
// State lives in the global zustand singleton (not lost when ContentPanel
// unmounts on panel switch; restored within the session) + native
// localStorage persistence (restored across restarts); see
// useEditorTabsStore for persistence details.
const EMPTY_TABS: EditorTab[] = [];

export function useEditorTabs(novelId: number) {
  const key = String(novelId);
  const tabs = useEditorTabsStore((s) => s.byNovel[key]?.tabs ?? EMPTY_TABS);
  const activeTabId = useEditorTabsStore(
    (s) => s.byNovel[key]?.activeTabId ?? null,
  );
  const activeTab = tabs.find((t) => t.id === activeTabId) ?? null;
  // The store is synchronously restored from localStorage at module load,
  // so tabs exist on the first render — always true.
  // initRef is kept only because ContentPanel's existing code destructures
  // it.
  const initRef = useRef(true);

  const openTab = useCallback(
    (tab: Omit<EditorTab, "id"> & { id?: string }) =>
      useEditorTabsStore.getState().openTab(novelId, tab),
    [novelId],
  );
  const closeTab = useCallback(
    (id: string) => useEditorTabsStore.getState().closeTab(novelId, id),
    [novelId],
  );
  const closeAllTabs = useCallback(
    () => useEditorTabsStore.getState().closeAllTabs(novelId),
    [novelId],
  );
  const setActiveTabId = useCallback(
    (id: string) => useEditorTabsStore.getState().setActiveTabId(novelId, id),
    [novelId],
  );
  const updateTab = useCallback(
    (id: string, patch: Partial<EditorTab>) =>
      useEditorTabsStore.getState().updateTab(novelId, id, patch),
    [novelId],
  );
  const openDiffTab = useCallback(
    (data: {
      path: string;
      title: string;
      diff: string;
      original: string;
      modified: string;
      changeType: string;
      reason: string;
      toolId: string;
    }) => useEditorTabsStore.getState().openDiffTab(novelId, data),
    [novelId],
  );

  return {
    tabs,
    activeTab,
    activeTabId,
    openTab,
    closeTab,
    closeAllTabs,
    setActiveTabId,
    updateTab,
    openDiffTab,
    initRef,
  };
}
