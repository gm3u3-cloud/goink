import { create } from "zustand";

// useEditorStore: editor/content-selection related cross-domain UI state.
// - tabTarget: highlight indicator for the chapter/goink selected in
//   SidePanel (consumed by ChapterList)
// - activeContent: current tab content in ContentPanel (consumed by the
//   StatusBar word count)
// - isDirty: dirty state of the current tab in ContentPanel (consumed by the
//   StatusBar save-status dot)
//
// All three states are shared across components (writer and reader live in
// different domain components), so they live in src/stores/.
// reset() is called automatically by WorkspaceView when it watches
// useNovelStore.activeNovelId change — after switching novels it clears the
// old novel's leftover highlight/word-count/dirty state, equivalent to the
// 3 setState lines inside the original switchToNovel wrapper.
interface EditorState {
  tabTarget: { path: string; title: string } | null;
  activeContent: string;
  isDirty: boolean;
  setTabTarget: (target: { path: string; title: string } | null) => void;
  setActiveContent: (content: string) => void;
  setIsDirty: (dirty: boolean) => void;
  reset: () => void;
}

export const useEditorStore = create<EditorState>((set) => ({
  tabTarget: null,
  activeContent: "",
  isDirty: false,
  setTabTarget: (target) => set({ tabTarget: target }),
  setActiveContent: (content) => set({ activeContent: content }),
  setIsDirty: (dirty) => set({ isDirty: dirty }),
  reset: () => set({ tabTarget: null, activeContent: "", isDirty: false }),
}));
