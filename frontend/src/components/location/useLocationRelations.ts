import { useQuery } from "@tanstack/react-query";
import { GetLocationRelations } from "@/lib/wailsjs/go/app/App";
import { locationKeys } from "@/lib/queryKeys";

// useLocationRelations: location spatial relation query
// (location.LocationRelation[]).
// enabled: !!novelId guard; no staleTime is set (inherits the global 30s,
// same as useNovels/useCharacters).
// Consumer: LocationGraph. After CRUD, the mutation's invalidateQueries
// keeps it in sync (mutations extracted in 4.2.2).
export function useLocationRelations(novelId: number) {
  return useQuery({
    queryKey: locationKeys.relations(novelId),
    queryFn: async () => {
      const list = await GetLocationRelations(novelId);
      return list ?? [];
    },
    enabled: !!novelId,
  });
}
