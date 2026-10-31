import { useQuery } from "@tanstack/react-query";
import { GetCharacters } from "@/lib/wailsjs/go/app/App";
import { characterKeys } from "@/lib/queryKeys";

// useCharacters: character list query.
// queryFn imports wailsjs GetCharacters directly (not useApp); no staleTime
// is set (inherits the global 30s, same as useNovels).
// enabled: !!novelId guard; no fetch when novelId=0 (data falls back to an
// empty array).
// Consumers: CharacterListView / CharacterGraph / CharacterList share the
// cache; after CRUD the mutation's invalidateQueries keeps them in sync
// (mutations extracted in 4.1.2).
export function useCharacters(novelId: number) {
  return useQuery({
    queryKey: characterKeys.list(novelId),
    queryFn: async () => {
      const list = await GetCharacters(novelId);
      return list ?? [];
    },
    enabled: !!novelId,
  });
}
