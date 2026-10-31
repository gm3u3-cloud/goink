import { useQuery } from "@tanstack/react-query";
import { GetLocations } from "@/lib/wailsjs/go/app/App";
import { locationKeys } from "@/lib/queryKeys";

// useLocations: location list query.
// queryFn imports wailsjs GetLocations directly (not useApp); no staleTime
// is set (inherits the global 30s, same as useNovels/useCharacters).
// enabled: !!novelId guard; no fetch when novelId=0 (data falls back to an
// empty array).
// Consumers: LocationListView / LocationList / LocationGraph share the
// cache; after CRUD the mutation's invalidateQueries keeps them in sync
// (mutations extracted in 4.2.2).
export function useLocations(novelId: number) {
  return useQuery({
    queryKey: locationKeys.list(novelId),
    queryFn: async () => {
      const list = await GetLocations(novelId);
      return list ?? [];
    },
    enabled: !!novelId,
  });
}
