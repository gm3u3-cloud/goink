import { useMutation, useQueryClient } from "@tanstack/react-query";
import { DeleteReaderPerspective } from "@/lib/wailsjs/go/app/App";
import { readerKeys } from "@/lib/queryKeys";

// useDeleteReaderPerspective: delete reader-perspective entry mutation.
// mutationFn directly imports the wailsjs DeleteReaderPerspective (no useApp).
// Note the parameter order: DeleteReaderPerspective(entryId, novelId), which
// is the reverse of timeline's DeleteTimelineEntry(novelId, entryId) (the
// reader backend signature is like this).
// Consumer: ReaderView.confirmDelete (mutateAsync errors are caught by the
// handler's try/catch).
// The handler is responsible for setDeleteTarget(null) + the error toast
// (side effects are not put in the mutation).
// onSuccess invalidates reader: the list syncs after the entry is deleted
// (ReaderView + ReaderList share the cache).
export function useDeleteReaderPerspective(novelId: number) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (entryId: number) => DeleteReaderPerspective(entryId, novelId),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: readerKeys.list(novelId) });
    },
  });
}
