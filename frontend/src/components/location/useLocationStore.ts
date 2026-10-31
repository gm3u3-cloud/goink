import { create } from "zustand";

// useLocationStore: UI state for the location domain (see
// useCharacterStore).
// Currently only holds deletingLocationId — for consolidated deletion:
// LocationList (sidebar) and LocationListView (main area) share the single
// ConfirmDialog + execution entry point (LocationListView), sharing the
// deletion target via the store.
// Component-internal states (editMode/form/viewTab, etc.) stay in the
// component and do not go into the store.
interface LocationStoreState {
  deletingLocationId: number | null;
  setDeletingLocationId: (id: number | null) => void;
}

export const useLocationStore = create<LocationStoreState>((set) => ({
  deletingLocationId: null,
  setDeletingLocationId: (id) => set({ deletingLocationId: id }),
}));
