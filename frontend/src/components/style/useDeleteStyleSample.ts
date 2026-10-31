import { useMutation, useQueryClient } from "@tanstack/react-query";
import { DeleteStyleSample } from "@/lib/wailsjs/go/app/App";
import type { app } from "@/lib/wailsjs/go/models";
import { styleSampleKeys } from "@/lib/queryKeys";

// useDeleteStyleSample: delete style sample mutation.
// mutationFn imports wailsjs DeleteStyleSample directly (not useApp).
// Input app.DeleteStyleSampleInput: { id }.
// The handler is responsible for the setSelected filter +
// setDeleteTarget(null) + throwing the error (the side effects differ, so
// they are not placed in the mutation).
// onSuccess invalidates styleSampleKeys.all: the list refreshes
// (StyleView/StyleSampleList stay in sync).
// The mutation has no onError; the caller does try/catch + toastError
// (aligned with the character pattern).
export function useDeleteStyleSample() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: app.DeleteStyleSampleInput) => DeleteStyleSample(input),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: styleSampleKeys.all });
    },
  });
}
