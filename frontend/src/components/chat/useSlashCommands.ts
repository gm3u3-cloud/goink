import { useQuery } from "@tanstack/react-query";
import { ListSlashCommands } from "@/lib/wailsjs/go/app/App";
import { slashCommandKeys } from "@/lib/queryKeys";

// useSlashCommands: slash command list query (consumed by ChatInput's
// SlashMenu).
// enabled: !!novelId guard; no fetch when novelId=0.
export function useSlashCommands(novelId: number) {
  return useQuery({
    queryKey: slashCommandKeys.list(novelId),
    queryFn: async () => {
      const list = await ListSlashCommands({ novel_id: novelId });
      return list ?? [];
    },
    enabled: !!novelId,
  });
}
