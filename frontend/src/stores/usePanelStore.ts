import { create } from "zustand";
import type { PanelId, SidebarPanelId } from "@/types/panel";

interface PanelState {
  activePanel: PanelId;
  sidebarPanel: SidebarPanelId | null;
  sidebarClosed: boolean;
  setActivePanel: (panel: PanelId) => void;
  setSidebarPanel: (panel: SidebarPanelId | null) => void;
  setSidebarClosed: (closed: boolean) => void;
}

// activePanel defaults to "novels"; WorkspaceView overrides it to "chapters"
// on mount using initialNovelId.
// The collapse/expand logic in handleActivitySelect is condition-based and is
// composed by the caller from the 3 setters; there is no toggleSidebar (there
// is no simple toggle semantics).
export const usePanelStore = create<PanelState>((set) => ({
  activePanel: "novels",
  sidebarPanel: null,
  sidebarClosed: false,
  setActivePanel: (panel) => set({ activePanel: panel }),
  setSidebarPanel: (panel) => set({ sidebarPanel: panel }),
  setSidebarClosed: (closed) => set({ sidebarClosed: closed }),
}));
