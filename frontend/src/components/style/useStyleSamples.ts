import { useQuery } from "@tanstack/react-query";
import { ListStyleSamples } from "@/lib/wailsjs/go/app/App";
import { styleSampleKeys } from "@/lib/queryKeys";

// useStyleSamples: single-page list query for style samples (StyleView
// previous/next pagination, PAGE_SIZE=15).
// queryKey includes page/size/search, distinguishing the cache from
// useInfiniteStyleSamples's infinite scroll cache.
// StyleSampleList's infinite scroll uses useInfiniteStyleSamples (page
// managed by pageParam).
// No enabled guard is set: novel_id=0 is a valid global samples query
// (is_global), so it always fetches.
// Consumer: StyleView; after CRUD the mutation's invalidateQueries keeps it
// in sync (mutation extracted in commit 2).
// 5.3 commit 1: GET errors are handled by the global middleware
// (queryErrorToast.ts); the component does not mount toastError.
export function useStyleSamples(input: {
  novelId: number;
  page: number;
  size: number;
  search: string;
}) {
  return useQuery({
    queryKey: styleSampleKeys.list(
      input.novelId,
      input.page,
      input.size,
      input.search,
    ),
    queryFn: async () => {
      const r = await ListStyleSamples({
        novel_id: input.novelId,
        page: input.page,
        size: input.size,
        search: input.search,
      });
      return r;
    },
  });
}
