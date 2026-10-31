import { useQuery } from "@tanstack/react-query";
import { GetWritingActivity } from "@/lib/wailsjs/go/app/App";
import { writingActivityKeys } from "@/lib/queryKeys";
import type { writing } from "@/lib/wailsjs/go/models";

// useWritingActivity: writing calendar for the past N months query (the
// ProfileView green-cells data).
// queryFn directly imports the wailsjs GetWritingActivity (no useApp).
// The data falls back to [] (an empty array when there are no writing
// records).
// GET errors are handled by the global middleware (queryErrorToast.ts); the
// component does not attach toastError.
// 5.7 commit 1.
export type WritingActivity = writing.DailyActivity;

export function useWritingActivity(months: number) {
  return useQuery({
    queryKey: writingActivityKeys.detail(months),
    queryFn: async () => {
      const r = (await GetWritingActivity(months)) as WritingActivity[];
      return r ?? [];
    },
  });
}
