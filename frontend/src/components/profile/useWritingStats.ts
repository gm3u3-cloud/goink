import { useQuery } from "@tanstack/react-query";
import { GetWritingStats } from "@/lib/wailsjs/go/app/App";
import { writingStatsKeys } from "@/lib/queryKeys";
import type { writing } from "@/lib/wailsjs/go/models";

// useWritingStats: global writing statistics query (total words / writing
// days / consecutive days / work count / chapter count).
// queryFn directly imports wailsjs GetWritingStats (no useApp).
// GET errors are handled by the global middleware (queryErrorToast.ts); the
// component does not attach a toastError.
// 5.7 commit 1.
export type WritingStats = writing.WritingStats;

export function useWritingStats() {
  return useQuery({
    queryKey: writingStatsKeys.all,
    queryFn: async () => {
      return (await GetWritingStats()) as WritingStats;
    },
  });
}
