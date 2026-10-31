import { useQuery } from "@tanstack/react-query";
import { GetSessionMessages } from "@/lib/wailsjs/go/app/App";
import { sessionMessagesKeys } from "@/lib/queryKeys";

// useSessionMessages: session history messages query (ChatPanel rebuilds
// turns from it via rebuildTurns).
// enabled: !!sid guard; no fetch when sid is empty (new conversation /
// nothing selected).
// Note: this query is only responsible for "first load / session-switch
// history restoration"; turn increments during streaming are still maintained
// by local state (the agent event subscription) and do not go through the
// query cache (5.1 special point 1).
export function useSessionMessages(sid: string) {
  return useQuery({
    queryKey: sessionMessagesKeys.detail(sid),
    queryFn: async () => {
      const msgs = await GetSessionMessages(sid);
      return msgs ?? [];
    },
    enabled: !!sid,
  });
}
