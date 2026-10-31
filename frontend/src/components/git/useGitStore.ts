import { create } from "zustand";
import type { git } from "@/lib/wailsjs/go/models";

// useGitStore: cross-component UI state internal to the git domain — the
// selected diff file.
// Writer: GitHistoryList (clicking a file in the sidebar list); reader:
// GitCommitView (diff rendering in the main area).
// Both are in the git domain, so per the "domain store lives in the domain
// directory" principle it is placed here (same pattern as useNovelStore).
// reset() is called automatically by WorkspaceView when
// useNovelStore.activeNovelId changes — it clears the previous novel's
// leftover diff after switching novels, equivalent to the old
// setSelectedGitFile(null) inside the switchToNovel wrapper.
interface GitState {
  selectedGitFile: git.FileDiff | null;
  setSelectedGitFile: (file: git.FileDiff | null) => void;
  reset: () => void;
}

export const useGitStore = create<GitState>((set) => ({
  selectedGitFile: null,
  setSelectedGitFile: (file) => set({ selectedGitFile: file }),
  reset: () => set({ selectedGitFile: null }),
}));
