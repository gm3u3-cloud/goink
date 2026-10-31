import { useQuery } from "@tanstack/react-query";
import { GetFileDiff } from "@/lib/wailsjs/go/app/App";
import { fileDiffKeys } from "@/lib/queryKeys";

// useFileDiff: the diff content for a single file (fetched on demand when
// the file is selected).
// enabled guard: only fetch when !!hash && !!filePath.
//
// 5.4 commit 5: GET errors are handled by the global middleware
// (queryErrorToast.ts); the component does not attach toastError.
// The diff data is derived from the query data; the component's useEffect
// watches for data changes and calls onSelectFile to pass it up to the parent.
export function useFileDiff(
  novelId: number,
  hash: string | null,
  filePath: string | null,
) {
  return useQuery({
    queryKey: fileDiffKeys.detail(novelId, hash ?? "", filePath ?? ""),
    queryFn: () => GetFileDiff(novelId, hash!, filePath!),
    enabled: !!hash && !!filePath,
  });
}
