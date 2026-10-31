import { useQuery } from "@tanstack/react-query";
import { SearchAll } from "@/lib/wailsjs/go/app/App";
import { searchKeys } from "@/lib/queryKeys";
import type { search } from "@/lib/wailsjs/go/models";

// useSearch: global cross-entity search query (SearchAll is the backend's
// unified entry point: cross-entity + body text + RAG).
// The queryKey includes the query string to drive refetch and let the
// query's built-in race protection take over (replaces the old reqIdRef).
// enabled guard: no fetch when novelId=0 or the query is empty.
// staleTime=0 (search is an action where the user actively expects the
// latest results; a new word always refetches).
// refetchOnMount=false + refetchOnWindowFocus=false: switching away from the
// search panel and back uses the cache without refetching, matching the old
// implementation where searchResults was kept permanently in the
// WorkspaceView holder and displayed directly without a spinner on return.
// gcTime=10min: covers normal panel-switching scenarios, avoiding a cache GC
// that would trigger a spinner on return.
// Consumer: SearchPanel; GET errors are handled by the global middleware
// (queryErrorToast.ts); the component does not attach toastError.
// 5.5 commit 1.
export type SearchResult = search.Result;

export function useSearch(input: { novelId: number; query: string }) {
  return useQuery({
    queryKey: searchKeys.list(input.novelId, input.query),
    queryFn: async () => {
      const r = (await SearchAll(
        input.novelId,
        input.query.trim(),
      )) as unknown as SearchResult[];
      return r ?? [];
    },
    enabled: !!input.novelId && !!input.query.trim(),
    staleTime: 0,
    gcTime: 600_000,
    refetchOnMount: false,
    refetchOnWindowFocus: false,
  });
}
