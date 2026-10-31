import { create } from "zustand";

interface SearchState {
  query: string;
  setQuery: (q: string) => void;
}

// 5.5 commit 2: the search query is lifted into a store, eliminating the
// WorkspaceView→SidePanel→SearchPanel prop-drilling chain.
// The store is held globally, so after SearchPanel unmounts and you switch
// back the query is still preserved (rule 7: UI/UX unchanged).
// Search results still live in the useQuery cache (commit 1 useSearch); this
// store only holds the query.
export const useSearchStore = create<SearchState>((set) => ({
  query: "",
  setQuery: (q) => set({ query: q }),
}));
