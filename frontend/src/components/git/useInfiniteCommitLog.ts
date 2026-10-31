import { useInfiniteQuery } from "@tanstack/react-query";
import { GetCommitLog } from "@/lib/wailsjs/go/app/App";
import { gitCommitKeys } from "@/lib/queryKeys";

// useInfiniteCommitLog: infinite-scroll query for the git commit history
// (GitHistoryList size=50).
//
// Unlike useInfiniteSessions/useInfiniteStyleSamples, git uses **cursor
// pagination** (afterHash) rather than page pagination — git log has no
// page number / total_pages concept; the commit history is a linked-list
// structure, so the next page can only be fetched using a hash as the
// cursor (the natural pagination of git log --after=<hash>).
//
// - initialPageParam="" means start fetching from HEAD
// - pageParam is the afterHash string (not in the queryKey; managed by
//   useInfiniteQuery)
// - getNextPageParam: when the previous page is full (== size), return the
//   hash of that page's last commit as the next-page cursor; otherwise
//   undefined (no more pages)
// - data.pages is an array of CommitInfo[] for each loaded page; consumers
//   flatMap(p => p) to get the accumulated list
//
// 5.4 commit 5: GET errors are handled by the global middleware
// (queryErrorToast.ts); the component does not attach toastError.
export function useInfiniteCommitLog(input: {
  novelId: number;
  size: number;
  enabled: boolean;
}) {
  return useInfiniteQuery({
    queryKey: gitCommitKeys.infiniteList(input.novelId, input.size),
    queryFn: async ({ pageParam }) => {
      const list = await GetCommitLog(
        input.novelId,
        input.size,
        pageParam ?? "",
      );
      return list ?? [];
    },
    initialPageParam: "",
    getNextPageParam: (lastPage) =>
      lastPage.length >= input.size
        ? lastPage[lastPage.length - 1].hash
        : undefined,
    enabled: input.enabled && !!input.novelId,
  });
}
