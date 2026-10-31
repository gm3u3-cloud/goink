import { useMutation, useQueryClient } from "@tanstack/react-query";
import { CreateTimelineEntry } from "@/lib/wailsjs/go/app/App";
import type { app } from "@/lib/wailsjs/go/models";
import { timelineKeys } from "@/lib/queryKeys";

// useCreateTimelineEntry: create timeline entry mutation.
// mutationFn imports wailsjs CreateTimelineEntry directly (not useApp) and
// returns timeline.TimelineEntry.
// The handler is responsible for setEditMode(null) + throwing the error
// (the side effects differ, so they are not placed in the mutation).
// onSuccess invalidates timeline: the new entry enters the list, and
// TimelineView / TimelineList stay in sync.
// chapter-plans/max-chapter are not invalidated: a newly created entry does
// not affect the plans or the novel's max chapter number.
export function useCreateTimelineEntry(novelId: number) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: app.CreateTimelineEntryInput) =>
      CreateTimelineEntry(novelId, input),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: timelineKeys.list(novelId) });
    },
  });
}
