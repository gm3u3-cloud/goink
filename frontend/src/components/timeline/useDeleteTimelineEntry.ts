import { useMutation, useQueryClient } from "@tanstack/react-query";
import { DeleteTimelineEntry } from "@/lib/wailsjs/go/app/App";
import { timelineKeys } from "@/lib/queryKeys";

// useDeleteTimelineEntry: delete timeline entry mutation.
// mutationFn imports wailsjs DeleteTimelineEntry directly (not useApp).
// Consumer: TimelineView.confirmDelete (the mutateAsync throw is caught by
// the handler's try/catch).
// The handler is responsible for setDeleteTarget(null) + the error toast
// (side effects are not placed in the mutation).
// onSuccess invalidates timeline: after the entry is deleted the list stays
// in sync (TimelineView + TimelineList share the cache).
// chapter-plans is not invalidated: plans are independent data; deleting an
// entry does not affect the plans.
// max-chapter is not invalidated: deleting an entry does not affect the
// novel's max chapter number.
export function useDeleteTimelineEntry(novelId: number) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (entryId: number) => DeleteTimelineEntry(novelId, entryId),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: timelineKeys.list(novelId) });
    },
  });
}
