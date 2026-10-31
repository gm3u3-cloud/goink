import { useMutation, useQueryClient } from "@tanstack/react-query";
import { UpdateStyleSample } from "@/lib/wailsjs/go/app/App";
import type { app } from "@/lib/wailsjs/go/models";
import { styleSampleKeys } from "@/lib/queryKeys";

// useUpdateStyleSample: update style sample mutation.
// mutationFn imports wailsjs UpdateStyleSample directly (not useApp).
// Input app.UpdateStyleSampleInput: id/name/content/tags/is_global/novel_id
// full payload (§6).
// The handler is responsible for setDetailId(null) + throwing the error
// (the side effects differ, so they are not placed in the mutation).
// onSuccess invalidates the list (all prefix) + the detail (current id):
// the list refreshes + the detail cache updates (the next open gets the new
// data).
// The mutation has no onError; the caller does try/catch + setError
// (aligned with the character pattern).
export function useUpdateStyleSample() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: app.UpdateStyleSampleInput) => UpdateStyleSample(input),
    onSuccess: (_data, variables) => {
      qc.invalidateQueries({ queryKey: styleSampleKeys.all });
      qc.invalidateQueries({ queryKey: styleSampleKeys.detail(variables.id) });
    },
  });
}
