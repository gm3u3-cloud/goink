import { useQuery } from "@tanstack/react-query";
import { GetCharacterRelations } from "@/lib/wailsjs/go/app/App";
import { characterKeys } from "@/lib/queryKeys";

// useCharacterRelations: character relationship graph query
// (character.CharacterRelation[]).
// enabled: !!novelId guard; no staleTime is set (inherits the global 30s,
// same as useNovels).
// Consumer: CharacterGraph. After CRUD, the mutation's invalidateQueries
// keeps it in sync (mutations extracted in 4.1.2).
export function useCharacterRelations(novelId: number) {
  return useQuery({
    queryKey: characterKeys.relations(novelId),
    queryFn: async () => {
      const list = await GetCharacterRelations(novelId);
      return list ?? [];
    },
    enabled: !!novelId,
  });
}
