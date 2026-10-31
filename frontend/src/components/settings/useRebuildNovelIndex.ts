import { useMutation } from "@tanstack/react-query";
import { RebuildNovelIndex } from "@/lib/wailsjs/go/app/App";

// useRebuildNovelIndex: mutation to rebuild the novel vector index (command
// operation).
// No invalidate needed on onSuccess (the search query has staleTime=0, so
// the user's next search auto-refetches; rebuilding the index does not
// change cached novel/chapter metadata).
// Error handling is done by the caller with try/catch + toastError (keeps
// the component-level toast, settings.rebuildFailed).
// 5.8 commit 2.
export function useRebuildNovelIndex() {
  return useMutation({
    mutationFn: (novelId: number) => RebuildNovelIndex(novelId),
  });
}
