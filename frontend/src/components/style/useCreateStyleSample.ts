import { useMutation, useQueryClient } from "@tanstack/react-query";
import { CreateStyleSample } from "@/lib/wailsjs/go/app/App";
import type { app } from "@/lib/wailsjs/go/models";
import { styleSampleKeys } from "@/lib/queryKeys";

// useCreateStyleSample: create style sample mutation.
// mutationFn imports wailsjs CreateStyleSample directly (not useApp).
// Input app.CreateStyleSampleInput: novel_id/is_global/name/content/tags.
// The handler is responsible for setNewName/reset form/setPage(1) +
// throwing the error (the side effects differ, so they are not placed in
// the mutation).
// onSuccess invalidates styleSampleKeys.all: the new sample enters the
// list, and the StyleView/StyleSampleList caches stay in sync.
// The mutation has no onError; the caller does try/catch + setError
// (aligned with the character pattern).
export function useCreateStyleSample() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: app.CreateStyleSampleInput) => CreateStyleSample(input),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: styleSampleKeys.all });
    },
  });
}
