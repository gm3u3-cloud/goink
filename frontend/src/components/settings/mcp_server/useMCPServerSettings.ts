import { useQuery } from "@tanstack/react-query";
import { GetMCPServerSettings } from "@/lib/wailsjs/go/app/App";
import { mcpServerKeys } from "@/lib/queryKeys";

export function useMCPServerSettings() {
  return useQuery({
    queryKey: mcpServerKeys.all,
    queryFn: GetMCPServerSettings,
    staleTime: 0,
    refetchInterval: (query) =>
      query.state.data?.status.stopping ? 1000 : false,
  });
}
