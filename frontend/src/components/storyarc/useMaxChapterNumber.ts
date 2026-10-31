import { useQuery } from "@tanstack/react-query";
import { GetMaxChapterNumber } from "@/lib/wailsjs/go/app/App";
import { maxChapterKeys } from "@/lib/queryKeys";

// useMaxChapterNumber: novel max chapter number query (number).
// Used to initialize the storyarc chapter window center windowCenter
// (shared by ArcListView / StoryArcGraph).
// queryFn imports wailsjs GetMaxChapterNumber directly (not useApp).
// enabled: !!novelId guard; no fetch when novelId=0 (data falls back to 0).
// After CRUD the mutation's invalidateQueries keeps it in sync (mutations
// extracted in commits 2/3).
export function useMaxChapterNumber(novelId: number) {
  return useQuery({
    queryKey: maxChapterKeys.detail(novelId),
    queryFn: async () => {
      const max = await GetMaxChapterNumber(novelId);
      return max ?? 0;
    },
    enabled: !!novelId,
  });
}
