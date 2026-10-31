import { useQuery } from "@tanstack/react-query";
import { GetSessions } from "@/lib/wailsjs/go/app/App";
import { sessionKeys } from "@/lib/queryKeys";

// useSessions: single-page query for the session list (ChatPanel recent
// sessions page=1 size=5).
// queryKey includes page/size/search, distinguishing the cache from
// SessionHistory's useInfiniteSessions.
// SessionHistory's infinite scroll uses useInfiniteSessions (page managed by
// pageParam).
// enabled: !!novelId guard; no fetch when novelId=0.
export function useSessions(input: {
  novelId: number;
  page: number;
  size: number;
  search: string;
}) {
  return useQuery({
    queryKey: sessionKeys.list(
      input.novelId,
      input.page,
      input.size,
      input.search,
    ),
    queryFn: async () => {
      const r = await GetSessions({
        novel_id: input.novelId,
        page: input.page,
        size: input.size,
        search: input.search,
      });
      return r;
    },
    enabled: !!input.novelId,
  });
}
