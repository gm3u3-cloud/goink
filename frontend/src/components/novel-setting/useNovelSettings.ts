import { useQuery } from "@tanstack/react-query";
import { GetNovelSettings } from "@/lib/wailsjs/go/app/App";
import { novelSettingKeys } from "@/lib/queryKeys";

// useNovelSettings: novel-setting list query (including items + token
// budget).
// The backend GetNovelSettings(novelId) returns app.SettingResult (with
// items/token_count/over_budget — a single items array, with no
// global/novel distinction, unlike preference's PreferenceResult two-array
// result).
// The queryKey uses ["novel-settings", novelId] for full caching; a single
// invalidate refreshes everything.
// NovelSettingView / NovelSettingList share the cache:
// - NovelSettingView reads result.items / result.token_count /
//   result.over_budget
// - NovelSettingList reads result.items
export function useNovelSettings(novelId: number) {
  return useQuery({
    queryKey: novelSettingKeys.list(novelId),
    queryFn: async () => {
      const result = await GetNovelSettings(novelId);
      return result;
    },
    enabled: !!novelId,
  });
}
