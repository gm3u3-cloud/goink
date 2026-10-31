import { useQuery } from "@tanstack/react-query";
import { GetReaderPerspectives } from "@/lib/wailsjs/go/app/App";
import { readerKeys } from "@/lib/queryKeys";

// useReaderPerspectives: reader-perspective entry list query.
// The backend GetReaderPerspective(novelId) is a single-parameter API (no
// chapter-window parameter, unlike timeline/storyarc).
// The queryKey uses ["reader", novelId] for full caching; a single
// invalidate refreshes everything.
// ReaderView / ReaderList share the cache (the same List+View pattern as
// character/location).
export function useReaderPerspectives(novelId: number) {
  return useQuery({
    queryKey: readerKeys.list(novelId),
    queryFn: async () => {
      const list = await GetReaderPerspectives(novelId);
      return list ?? [];
    },
    enabled: !!novelId,
  });
}
