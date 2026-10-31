import { useQuery } from "@tanstack/react-query";
import { GetChapterPlans } from "@/lib/wailsjs/go/app/App";
import { chapterPlanKeys } from "@/lib/queryKeys";

// useChapterPlans: chapter plan 3-slot (next/near/far) query.
// GetChapterPlans(novelId) is an independent API, parallel to
// useTimelineEntries (their invalidations do not affect each other).
// Consumed only by TimelineView (shown in the planTab).
export function useChapterPlans(novelId: number) {
  return useQuery({
    queryKey: chapterPlanKeys.list(novelId),
    queryFn: async () => {
      const list = await GetChapterPlans(novelId);
      return list ?? [];
    },
    enabled: !!novelId,
  });
}
