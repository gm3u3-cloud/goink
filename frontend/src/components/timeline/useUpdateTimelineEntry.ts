import { useMutation, useQueryClient } from "@tanstack/react-query";
import { UpdateTimelineEntry } from "@/lib/wailsjs/go/app/App";
import type { app } from "@/lib/wailsjs/go/models";
import { timelineKeys } from "@/lib/queryKeys";

// useUpdateTimelineEntry: update timeline entry mutation.
// mutationFn imports wailsjs UpdateTimelineEntry directly (not useApp).
// Input {id, input}: input uses app.UpdateTimelineEntryInput (PUT
// semantics, full payload), including handleQuickStatus's full payload.
// The handler is responsible for setEditMode(null) + throwing the error
// (the side effects differ, so they are not placed in the mutation).
// onSuccess invalidates timeline: the entry field changes enter the list.
// chapter-plans/max-chapter are not invalidated: entry fields do not affect
// the plans or the novel's max chapter number.
export function useUpdateTimelineEntry(novelId: number) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({
      id,
      input,
    }: {
      id: number;
      input: app.UpdateTimelineEntryInput;
    }) => UpdateTimelineEntry(novelId, id, input),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: timelineKeys.list(novelId) });
    },
  });
}
