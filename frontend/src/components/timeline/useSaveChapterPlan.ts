import { useMutation, useQueryClient } from "@tanstack/react-query";
import { UpdateChapterPlan } from "@/lib/wailsjs/go/app/App";
import type { app } from "@/lib/wailsjs/go/models";
import { chapterPlanKeys } from "@/lib/queryKeys";

// useSaveChapterPlan: save chapter plan mutation (plan 3-slot
// next/near/far).
// mutationFn imports wailsjs UpdateChapterPlan directly (not useApp).
// The backend UpdateChapterPlan upserts by scope (one each for
// next/near/far); there is no separate Create API.
// The handler is responsible for setEditMode(null) + throwing the error
// (the side effects differ, so they are not placed in the mutation).
// onSuccess invalidates chapter-plans: after the plan content changes, the
// plan area refreshes.
// timeline/max-chapter are not invalidated: plans do not affect the entry
// list or the novel's max chapter number.
export function useSaveChapterPlan(novelId: number) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: app.UpdateChapterPlanInput) =>
      UpdateChapterPlan(novelId, input),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: chapterPlanKeys.list(novelId) });
    },
  });
}
