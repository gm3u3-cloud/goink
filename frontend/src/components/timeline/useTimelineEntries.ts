import { useQuery } from "@tanstack/react-query";
import { GetTimelineEntries } from "@/lib/wailsjs/go/app/App";
import { timelineKeys } from "@/lib/queryKeys";

// useTimelineEntries: timeline entry list query.
// The backend GetTimelineEntries(novelId) fetches the full list (4b:
// deprecated the from/to chapter window; the frontend switches the window
// in memory).
// queryKey uses the ["timeline", novelId] full cache; one invalidate
// refreshes everything.
// TimelineView / TimelineList share the cache.
export function useTimelineEntries(novelId: number) {
  return useQuery({
    queryKey: timelineKeys.list(novelId),
    queryFn: async () => {
      const list = await GetTimelineEntries(novelId);
      return list ?? [];
    },
    enabled: !!novelId,
  });
}
