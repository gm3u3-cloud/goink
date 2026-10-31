import { useQuery } from "@tanstack/react-query";
import { GetChapters } from "@/lib/wailsjs/go/app/App";
import { chapterKeys } from "@/lib/queryKeys";

// useChapters: chapter metadata list query.
// queryFn directly imports wailsjs GetChapters (no useApp); no staleTime set
// (inherits the global 30s).
// enabled: the !!novelId guard skips fetching when novelId=0 (data falls back
// to an empty array).
// Consumer: ChapterList (the sidebar chapter list). After CRUD, the mutation's
// invalidateQueries keeps it in sync (5.2 commit 2).
// The chapter refresh triggered by file:changed events is handled by
// qc.invalidateQueries starting from commit 3.
export function useChapters(novelId: number) {
  return useQuery({
    queryKey: chapterKeys.list(novelId),
    queryFn: async () => {
      const list = await GetChapters(novelId);
      return list ?? [];
    },
    enabled: !!novelId,
  });
}
