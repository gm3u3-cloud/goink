import { useQuery } from "@tanstack/react-query";
import { GetSettings } from "@/lib/wailsjs/go/app/App";
import { settingsKeys } from "@/lib/queryKeys";
import type { config } from "@/lib/wailsjs/go/models";

// useProfileSettings: app settings query (profile consumes the user_name /
// avatar fields).
// Reuses settingsKeys.all and shares the cache with chat's useSettings —
// GetSettings is global settings; chat reads fields like selected_model_key /
// reasoning_effort, profile reads user_name / avatar, each taking its own
// fields, and TanStack Query only fetches once.
// queryFn directly imports the wailsjs GetSettings (no useApp).
// GET errors are handled by the global middleware (settings prefix →
// chat.settingsLoadFailed, generic wording).
// 5.7 commit 1.
export type ProfileSettings = config.AppSettings;

export function useProfileSettings() {
  return useQuery({
    queryKey: settingsKeys.all,
    queryFn: async () => {
      return (await GetSettings()) as ProfileSettings;
    },
  });
}
