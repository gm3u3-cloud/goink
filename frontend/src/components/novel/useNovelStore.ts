import { create } from "zustand";
import type { novel } from "@/lib/wailsjs/go/models";
import { SetActiveNovel } from "@/lib/wailsjs/go/app/App";

// useNovelStore: novel-domain UI state (activeNovelId + 4 dialog toggles) +
// the switchNovel action.
// Data goes through the useNovels query; CRUD goes through mutations
// (3.3-3.5); this store manages UI coordination state + the novel-switching
// action.
//
// switchNovel action (3.7): only handles the novel state + the backend call
// (SetActiveNovel).
// ContentPanel's tab switching is handled automatically inside useEditorTabs
// by subscribing to novelId (not in this action).
// The tabTarget/activeContent/selectedGitFile resets stay in the
// WorkspaceView thin wrapper (belonging to SidePanel/StatusBar/GitCommitView).
//
// Reason for not placing it in src/stores/: domain aggregation
// (00-conventions §4) — novel-domain-specific state lives in the same
// directory as useNovels/NovelDialogs. src/stores/ only holds cross-domain
// app-level stores (panel/focus/tab).
interface NovelUIState {
  activeNovelId: number;
  editingNovel: novel.Novel | null;
  deletingNovel: novel.Novel | null;
  showCreateDialog: boolean;
  exportNovelId: number | null;
  setActiveNovelId: (id: number) => void;
  setEditingNovel: (n: novel.Novel | null) => void;
  setDeletingNovel: (n: novel.Novel | null) => void;
  setShowCreateDialog: (b: boolean) => void;
  setExportNovelId: (id: number | null) => void;
  // Switch novel: set activeNovelId + notify the backend. ContentPanel/tab
  // resets are not handled by this action.
  switchNovel: (id: number) => Promise<void>;
}

export const useNovelStore = create<NovelUIState>((set) => {
  let switchQueue: Promise<void> = Promise.resolve();
  return {
    activeNovelId: 0,
    editingNovel: null,
    deletingNovel: null,
    showCreateDialog: false,
    exportNovelId: null,
    setActiveNovelId: (id) => set({ activeNovelId: id }),
    setEditingNovel: (n) => set({ editingNovel: n }),
    setDeletingNovel: (n) => set({ deletingNovel: n }),
    setShowCreateDialog: (b) => set({ showCreateDialog: b }),
    setExportNovelId: (id) => set({ exportNovelId: id }),
    switchNovel: (id) => {
      const switching = switchQueue.then(async () => {
        await SetActiveNovel({ novel_id: id });
        set({ activeNovelId: id });
      });
      switchQueue = switching.catch(() => undefined);
      return switching;
    },
  };
});
