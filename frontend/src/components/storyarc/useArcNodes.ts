import { useQuery } from "@tanstack/react-query";
import { GetArcNodes } from "@/lib/wailsjs/go/app/App";
import { arcNodeKeys } from "@/lib/queryKeys";

// useArcNodes: full arc-node list query.
// 4b: the GetArcNodes signature changed to a single parameter (deprecated
// fromChapter/toChapter; the backend now calls ListNodesByNovel(Size=-1)
// for the full list).
// enabled: !!novelId guard; no fetch when novelId=0 (data falls back to an
// empty array).
// Consumers: ArcListView / StoryArcGraph / ArcList share the cache; after
// CRUD the mutation's invalidateQueries keeps them in sync (mutations
// extracted in commits 2/3).
export function useArcNodes(novelId: number) {
  return useQuery({
    queryKey: arcNodeKeys.list(novelId),
    queryFn: async () => {
      const list = await GetArcNodes(novelId);
      return list ?? [];
    },
    enabled: !!novelId,
  });
}
