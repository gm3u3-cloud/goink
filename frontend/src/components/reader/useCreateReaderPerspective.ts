import { useMutation, useQueryClient } from "@tanstack/react-query";
import { CreateReaderPerspective } from "@/lib/wailsjs/go/app/App";
import type { app } from "@/lib/wailsjs/go/models";
import { readerKeys } from "@/lib/queryKeys";

// useCreateReaderPerspective: create reader perspective entry mutation.
// mutationFn imports wailsjs CreateReaderPerspective directly (not useApp)
// and returns reader.ReaderPerspective.
// Argument order: CreateReaderPerspective(novelId, input) (same as
// timeline).
// Consumer: ReaderView.handleCreate (the mutateAsync throw is caught by
// the handler's try/catch).
// The handler is responsible for setEditMode(null) + setForm(EMPTY_FORM) +
// setExpandedId(created.id) + the error toast (the side effects differ, so
// they are not placed in the mutation).
// onSuccess invalidates reader: the new entry enters the list, and
// ReaderView / ReaderList stay in sync.
export function useCreateReaderPerspective(novelId: number) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: app.CreateReaderPerspectiveInput) =>
      CreateReaderPerspective(novelId, input),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: readerKeys.list(novelId) });
    },
  });
}
