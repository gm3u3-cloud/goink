import { useMutation, useQueryClient } from "@tanstack/react-query";
import { UpdateReaderPerspective } from "@/lib/wailsjs/go/app/App";
import type { app } from "@/lib/wailsjs/go/models";
import { readerKeys } from "@/lib/queryKeys";

// useUpdateReaderPerspective: update reader-perspective entry mutation.
// mutationFn directly imports the wailsjs UpdateReaderPerspective (no useApp).
// Parameter order: UpdateReaderPerspective(entryId, novelId, input)
// (3 parameters, entryId first — different from timeline's
// UpdateTimelineEntry(novelId, id, input); the reader backend signature is
// like this).
// Input {id, input}: input uses app.UpdateReaderPerspectiveInput (PUT
// semantics, full payload sent back), including handleQuickReveal's full
// send-back.
// Consumers: ReaderView.handleUpdate / handleQuickReveal (mutateAsync
// errors are caught by the handler's try/catch).
// The handler is responsible for setEditMode(null) / the error toast
// (side effects vary, so they are not put in the mutation).
// onSuccess invalidates reader: the entry's field changes are reflected in
// the list.
export function useUpdateReaderPerspective(novelId: number) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({
      id,
      input,
    }: {
      id: number;
      input: app.UpdateReaderPerspectiveInput;
    }) => UpdateReaderPerspective(id, novelId, input),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: readerKeys.list(novelId) });
    },
  });
}
