import { useQuery } from "@tanstack/react-query";
import { GetPreferences } from "@/lib/wailsjs/go/app/App";
import { preferenceKeys } from "@/lib/queryKeys";

// usePreferences: preference list query (including the global + novel-level
// groups + token budget).
// The backend GetPreferences(novelId) returns app.PreferenceResult (with
// global/novel/token_count/over_budget) — not an array, unlike
// reader/timeline's array returns. The queryKey uses ["preferences", novelId]
// for full caching; a single invalidate refreshes everything.
// PreferenceView / PreferenceList share the cache:
// - PreferenceView reads result.global / result.novel / result.token_count /
//   result.over_budget
// - PreferenceList merges result.global + result.novel into a single list
export function usePreferences(novelId: number) {
  return useQuery({
    queryKey: preferenceKeys.list(novelId),
    queryFn: async () => {
      const result = await GetPreferences(novelId);
      return result;
    },
    enabled: !!novelId,
  });
}
