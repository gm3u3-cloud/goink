import { useQuery } from "@tanstack/react-query";
import { GetSession } from "@/lib/wailsjs/go/app/App";
import { sessionKeys } from "@/lib/queryKeys";

// useSession: single session details query (includes the usage field;
// ChatPanel restores lastUsage from it).
// enabled: !!sid guard; no fetch when sid is empty (new conversation /
// nothing selected).
// session_id is a string (the type was already fixed in
// queryKeys.sessionKeys.detail).
export function useSession(sid: string) {
  return useQuery({
    queryKey: sessionKeys.detail(sid),
    queryFn: async () => {
      const detail = await GetSession(sid);
      return detail;
    },
    enabled: !!sid,
  });
}
