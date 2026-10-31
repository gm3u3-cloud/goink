import { create } from "zustand";
import { isContentPath, type EditorTab } from "@/components/content/types";

// useEditorTabsStore: global in-memory singleton for the editor tab set +
// persistence.
//
// Background (issue #46): ContentPanel unmounts entirely when switching
// panels. Previously useEditorTabs used useState local state + writing to
// localStorage only on beforeunload, which caused:
//  1. Leaving and returning to the panel within a session lost the tab list
//     (it restored the old tab set from the last app close);
//  2. Scroll position (Monaco viewState / outline scrollTop) was not
//     remembered;
//  3. ContentEditor had no key, so all tabs shared one Monaco instance and
//     the scroll position was shared.
//
// Solution: a plain zustand store (in-memory singleton, state survives
// unmount) + hand-written native localStorage reads/writes
// (debounced + batch write on beforeunload + a version-number field).
// - content/isDirty are not persisted (refetched across restarts, same as
//   before); nor are diff tabs (the temporary approval flow).
// - Positions are keyed by `novelId:path:mode`: tabId is regenerated each
//   session and cannot be a persistence key; path is the real file path for
//   each mode (body=chapters/NNN.md, outline=outlines/NNN.md), which
//   naturally fits the future unified container tab model (a sub tab is just
//   an independent path). Schema changes go through version-number
//   migration.

const STORAGE_KEY = "goink_tabs_all";
const STORAGE_VERSION = 2;
const PERSIST_DEBOUNCE_MS = 300;

let idSeq = 0;
function nextId(type: EditorTab["type"]): string {
  return `${type}_${++idSeq}`;
}

// TabMeta: the persisted tab fields (excluding runtime content / diff, etc.).
type TabMeta = Pick<
  EditorTab,
  "path" | "outlinePath" | "title" | "type" | "viewMode" | "readOnly"
>;

// ReadingPosition: the reading position for a single (novelId, path, mode).
// - viewState: the output of Monaco editor.saveViewState() (content /
//   outline-edit modes)
// - scrollTop: the outline Markdown view scroll position (outline mode)
export interface ReadingPosition {
  scrollTop?: number;
  viewState?: unknown;
  updatedAt: number;
}

interface NovelTabs {
  tabs: EditorTab[];
  activeTabId: string | null;
}

interface EditorTabsStoreState {
  byNovel: Record<string, NovelTabs>;
  positions: Record<string, ReadingPosition>;
  openTab: (
    novelId: number,
    tab: Omit<EditorTab, "id"> & { id?: string },
  ) => string;
  closeTab: (novelId: number, id: string) => void;
  closeAllTabs: (novelId: number) => void;
  setActiveTabId: (novelId: number, id: string) => void;
  updateTab: (novelId: number, id: string, patch: Partial<EditorTab>) => void;
  openDiffTab: (
    novelId: number,
    data: {
      path: string;
      title: string;
      diff: string;
      original: string;
      modified: string;
      changeType: string;
      reason: string;
      toolId: string;
    },
  ) => string;
  setPosition: (key: string, pos: ReadingPosition) => void;
}

// ── Persistence ├─ Serialization ──

type PersistedNovel = { tabs: TabMeta[]; activePath: string | null };
type PersistedEnvelope = {
  version: number;
  byNovel: Record<string, PersistedNovel>;
  positions: Record<string, ReadingPosition>;
};

function toPersisted(state: EditorTabsStoreState): PersistedEnvelope {
  const byNovel: Record<string, PersistedNovel> = {};
  for (const [key, entry] of Object.entries(state.byNovel)) {
    const metas: TabMeta[] = entry.tabs
      .filter((t) => t.type !== "diff")
      .map((t) => ({
        path: t.path,
        outlinePath: t.outlinePath,
        title: t.title,
        type: t.type,
        viewMode: t.viewMode,
        readOnly: t.readOnly,
      }));
    if (metas.length === 0) continue;
    const activeTab = entry.tabs.find((t) => t.id === entry.activeTabId);
    byNovel[key] = {
      tabs: metas,
      activePath:
        activeTab && activeTab.type !== "diff" ? activeTab.path : null,
    };
  }
  return {
    version: STORAGE_VERSION,
    byNovel,
    positions: state.positions,
  };
}

// ── Persistence ├─ Deserialization (version mismatch / parse failure →
// empty) ──

const EMPTY_BYNOVEL: Record<string, NovelTabs> = {};

function loadInitial(): {
  byNovel: Record<string, NovelTabs>;
  positions: Record<string, ReadingPosition>;
} {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (!raw) return { byNovel: EMPTY_BYNOVEL, positions: {} };
    const data = JSON.parse(raw) as PersistedEnvelope;
    if (!data || data.version !== STORAGE_VERSION) {
      return { byNovel: EMPTY_BYNOVEL, positions: {} };
    }
    const byNovel: Record<string, NovelTabs> = {};
    for (const [key, entry] of Object.entries(data.byNovel || {})) {
      const tabs: EditorTab[] = (entry.tabs || [])
        .filter((t) => t && t.type !== "diff")
        .map((t) => ({ ...t, id: nextId(t.type) }));
      if (tabs.length === 0) continue;
      let activeTabId: string | null = null;
      if (entry.activePath) {
        const active = tabs.find((t) => t.path === entry.activePath);
        if (active) activeTabId = active.id;
      }
      if (!activeTabId) activeTabId = tabs[0].id;
      byNovel[key] = { tabs, activeTabId };
    }
    return { byNovel, positions: data.positions || {} };
  } catch {
    return { byNovel: EMPTY_BYNOVEL, positions: {} };
  }
}

const initial = loadInitial();

// ── Persistence ├─ Debounce + batch write on beforeunload ──

let persistTimer: ReturnType<typeof setTimeout> | null = null;

function schedulePersist() {
  if (persistTimer) clearTimeout(persistTimer);
  persistTimer = setTimeout(flushPersist, PERSIST_DEBOUNCE_MS);
}

function flushPersist() {
  if (persistTimer) {
    clearTimeout(persistTimer);
    persistTimer = null;
  }
  try {
    localStorage.setItem(
      STORAGE_KEY,
      JSON.stringify(toPersisted(useEditorTabsStore.getState())),
    );
  } catch {
    /* ignored */
  }
}

// withNovel: do a partial update of a specific novel's entry (start from an
// empty set if there is no entry).
function withNovel(
  key: string,
  updater: (entry: NovelTabs) => NovelTabs,
): (s: EditorTabsStoreState) => Pick<EditorTabsStoreState, "byNovel"> {
  return (s) => ({
    byNovel: {
      ...s.byNovel,
      [key]: updater(s.byNovel[key] ?? { tabs: [], activeTabId: null }),
    },
  });
}

// positionKeysForTab: all position keys that need to be cleaned up when this
// tab is closed.
// Consistent with the key-naming rules in ContentPanel.positionKeyFor:
//   - Body: `novelId:chapters/NNN.md:content`
//   - Outline / outline edit: `novelId:outlines/NNN.md:outline` /
//     `:outline-edit`
function positionKeysForTab(
  novelId: number,
  tab: Pick<EditorTab, "path" | "outlinePath">,
): string[] {
  const ns = `${novelId}:`;
  const keys = new Set<string>([`${ns}${tab.path}:content`]);
  // Only chapters/NNN.md (excluding goink.md) has a corresponding outline
  // file key
  if (isContentPath(tab.path) && tab.path !== "goink.md" && tab.outlinePath) {
    keys.add(`${ns}${tab.outlinePath}:outline`);
    keys.add(`${ns}${tab.outlinePath}:outline-edit`);
  }
  return [...keys];
}

export const useEditorTabsStore = create<EditorTabsStoreState>((set, get) => ({
  byNovel: initial.byNovel,
  positions: initial.positions,

  openTab: (novelId, tab) => {
    const key = String(novelId);
    const id = tab.id ?? nextId(tab.type);
    const existing = get().byNovel[key]?.tabs.find(
      (t) => t.path === tab.path && t.type === tab.type,
    );
    if (existing) {
      set(withNovel(key, (e) => ({ ...e, activeTabId: existing.id })));
      return existing.id;
    }
    const newTab: EditorTab = { ...tab, id };
    set(
      withNovel(key, (e) => ({
        ...e,
        tabs: [...e.tabs, newTab],
        activeTabId: id,
      })),
    );
    return id;
  },

  closeTab: (novelId, id) => {
    const key = String(novelId);
    const entry = get().byNovel[key];
    if (!entry || entry.tabs.length === 0) return;
    const closing = entry.tabs.find((t) => t.id === id);
    set((s) => {
      const nextByNovel = withNovel(key, (e) => {
        if (e.tabs.length <= 1) return { tabs: [], activeTabId: null };
        const idx = e.tabs.findIndex((t) => t.id === id);
        const next = e.tabs.filter((t) => t.id !== id);
        let activeTabId = e.activeTabId;
        if (e.activeTabId === id) {
          const newIdx = Math.min(idx, next.length - 1);
          activeTabId = next[newIdx]?.id ?? null;
        }
        return { tabs: next, activeTabId };
      })(s);
      const positions = { ...s.positions };
      if (closing?.type === "file") {
        for (const k of positionKeysForTab(novelId, closing)) {
          delete positions[k];
        }
      }
      return { byNovel: nextByNovel.byNovel, positions };
    });
  },

  closeAllTabs: (novelId) => {
    const key = String(novelId);
    const prefix = `${novelId}:`;
    set((s) => {
      const next = { ...s.byNovel };
      delete next[key];
      const positions: Record<string, ReadingPosition> = {};
      for (const [k, v] of Object.entries(s.positions)) {
        if (!k.startsWith(prefix)) positions[k] = v;
      }
      return { byNovel: next, positions };
    });
  },

  setActiveTabId: (novelId, id) => {
    set(withNovel(String(novelId), (e) => ({ ...e, activeTabId: id })));
  },

  updateTab: (novelId, id, patch) => {
    set(
      withNovel(String(novelId), (e) => ({
        ...e,
        tabs: e.tabs.map((t) => (t.id === id ? { ...t, ...patch } : t)),
      })),
    );
  },

  openDiffTab: (novelId, data) => {
    const key = String(novelId);
    const id = nextId("diff");
    set(
      withNovel(key, (e) => ({
        ...e,
        tabs: [...e.tabs, { id, type: "diff", ...data }],
        activeTabId: id,
      })),
    );
    return id;
  },

  setPosition: (key, pos) =>
    set((s) => ({ positions: { ...s.positions, [key]: pos } })),
}));

useEditorTabsStore.subscribe(() => schedulePersist());

if (typeof window !== "undefined") {
  window.addEventListener("beforeunload", flushPersist);
}
