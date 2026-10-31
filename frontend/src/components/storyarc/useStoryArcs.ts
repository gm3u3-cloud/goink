import { useQuery } from "@tanstack/react-query";
import { GetStoryArcs } from "@/lib/wailsjs/go/app/App";
import { storyarcKeys } from "@/lib/queryKeys";

// useStoryArcs: narrative arc list query.
// queryFn imports wailsjs GetStoryArcs directly (not useApp); no staleTime
// is set (inherits the global 30s, same as useNovels/useCharacters).
// enabled: !!novelId guard; no fetch when novelId=0 (data falls back to an
// empty array).
// Consumers: ArcList / ArcListView / StoryArcGraph share the cache; after
// CRUD the mutation's invalidateQueries keeps them in sync (mutations
// extracted in commits 2/3).
export function useStoryArcs(novelId: number) {
  return useQuery({
    queryKey: storyarcKeys.list(novelId),
    queryFn: async () => {
      const list = await GetStoryArcs(novelId);
      return list ?? [];
    },
    enabled: !!novelId,
  });
}
