import { useQuery } from "@tanstack/react-query";
import { GetSettings } from "@/lib/wailsjs/go/app/App";
import { settingsKeys } from "@/lib/queryKeys";

// useSettings: app settings query (includes persisted fields such as
// selected_model_key / reasoning_effort / approval_mode / last_session_id).
// queryFn imports wailsjs GetSettings directly (not useApp).
// ChatPanel's selected-state restoration depends on both this query data and
// the useModels data being ready.
export function useSettings() {
  return useQuery({
    queryKey: settingsKeys.all,
    queryFn: async () => {
      const settings = await GetSettings();
      return settings;
    },
  });
}
