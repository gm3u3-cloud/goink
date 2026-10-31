import { create } from "zustand";

type ViewTab = "list" | "graph";
interface CharacterGroupUIState {
  selectedGroups: Record<number, number | null>;
  viewTabs: Record<number, ViewTab>;
  editingGroups: Record<number, number | null | undefined>;
  deletingGroups: Record<number, number | undefined>;
  selectGroup: (novelId: number, groupId: number | null) => void;
  setViewTab: (novelId: number, tab: ViewTab) => void;
  editGroup: (novelId: number, groupId: number | null | undefined) => void;
  deleteGroup: (novelId: number, groupId: number | undefined) => void;
}

// Only shares the sidebar/main-area navigation and group edit requests;
// isolated per novel; the data is still managed by queries.
export const useCharacterGroupStore = create<CharacterGroupUIState>((set) => ({
  selectedGroups: {},
  viewTabs: {},
  editingGroups: {},
  deletingGroups: {},
  selectGroup: (novelId, groupId) =>
    set((state) => ({
      selectedGroups: { ...state.selectedGroups, [novelId]: groupId },
      viewTabs: { ...state.viewTabs, [novelId]: "list" },
    })),
  setViewTab: (novelId, tab) =>
    set((state) => ({
      viewTabs: { ...state.viewTabs, [novelId]: tab },
    })),
  editGroup: (novelId, groupId) =>
    set((state) => ({
      editingGroups: { ...state.editingGroups, [novelId]: groupId },
      viewTabs: { ...state.viewTabs, [novelId]: "list" },
    })),
  deleteGroup: (novelId, groupId) =>
    set((state) => ({
      deletingGroups: { ...state.deletingGroups, [novelId]: groupId },
      viewTabs: { ...state.viewTabs, [novelId]: "list" },
    })),
}));
