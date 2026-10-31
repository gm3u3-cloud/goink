import { useQuery } from "@tanstack/react-query";
import { GetCommitFileList } from "@/lib/wailsjs/go/app/App";
import { commitFileKeys } from "@/lib/queryKeys";

// useCommitFiles: the file list for a single commit (fetched on demand when
// the commit is expanded).
// enabled guard: only fetch when !!hash (no query when hash is empty).
// Returns FileEntry[] (already unwrapped from CommitFileListResult.files).
//
// 5.4 commit 5: GET errors are handled by the global middleware
// (queryErrorToast.ts); the component does not attach toastError, but the
// expandedError inline display is kept (the specific message for a failed
// commit expansion is shown inline by the component reading isError).
export function useCommitFiles(novelId: number, hash: string | null) {
  return useQuery({
    queryKey: commitFileKeys.list(novelId, hash ?? ""),
    queryFn: async () => {
      const result = await GetCommitFileList(novelId, hash!);
      return result?.files ?? [];
    },
    enabled: !!hash,
  });
}
