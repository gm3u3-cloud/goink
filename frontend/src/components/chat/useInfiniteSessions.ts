import { useInfiniteQuery } from "@tanstack/react-query";
import { GetSessions } from "@/lib/wailsjs/go/app/App";
import { sessionKeys } from "@/lib/queryKeys";

// useInfiniteSessions: infinite scroll query for the session list
// (SessionHistory size=20 + search).
// page is managed by useInfiniteQuery's pageParam (not in queryKey); a
// search change triggers a new query.
// data.pages is the array of PageResults for the loaded pages; consumers
// flatMap(p => p.items) to get the accumulated list.
// getNextPageParam: returns the next page's pageParam when the current page
// < total pages, otherwise undefined (no more pages).
// enabled: open && !!novelId (no fetch while the panel is closed).
export function useInfiniteSessions(input: {
  novelId: number;
  size: number;
  search: string;
  enabled: boolean;
}) {
  return useInfiniteQuery({
    queryKey: sessionKeys.infiniteList(input.novelId, input.size, input.search),
    queryFn: async ({ pageParam }) => {
      const r = await GetSessions({
        novel_id: input.novelId,
        page: pageParam,
        size: input.size,
        search: input.search,
      });
      return r;
    },
    initialPageParam: 1,
    getNextPageParam: (lastPage) =>
      lastPage.page < lastPage.total_pages ? lastPage.page + 1 : undefined,
    enabled: input.enabled && !!input.novelId,
  });
}
