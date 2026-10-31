import { useQuery } from "@tanstack/react-query";
import { GetModels } from "@/lib/wailsjs/go/app/App";
import { modelKeys } from "@/lib/queryKeys";

// useModels: available model list query.
// queryFn imports wailsjs GetModels directly (not useApp).
// Global config (no novelId dimension); ChatPanel is the first consumer; the
// cache will be shared when the settings domain is migrated later.
// No staleTime is set (inherits the global 30s); ChatPanel's selected-state
// restoration depends on this query data being ready.
export function useModels() {
  return useQuery({
    queryKey: modelKeys.all,
    queryFn: async () => {
      const list = await GetModels();
      return list ?? [];
    },
  });
}
