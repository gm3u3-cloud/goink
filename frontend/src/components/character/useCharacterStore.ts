import { create } from "zustand";

// useCharacterStore: the deletion-target state shared across components in
// the character domain.
// Only holds deletingCharacterId — shared between the main-area
// CharacterListView (mounts the single ConfirmDialog + executes the delete)
// and the sidebar CharacterList (delete clicks only dispatch), merging into
// a single deletion confirmation flow.
// Component-internal states like editMode/form/viewTab/search do not go into
// the store (README general principle 10).
// Data goes through the useCharacters query; create/update/delete go through
// mutations (4.1.2).
interface CharacterUIState {
  deletingCharacterId: number | null;
  setDeletingCharacterId: (id: number | null) => void;
}

export const useCharacterStore = create<CharacterUIState>((set) => ({
  deletingCharacterId: null,
  setDeletingCharacterId: (id) => set({ deletingCharacterId: id }),
}));
