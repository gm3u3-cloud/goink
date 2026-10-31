import { useMutation, useQueryClient } from "@tanstack/react-query";
import { UpdateStoryArc } from "@/lib/wailsjs/go/app/App";
import type { app } from "@/lib/wailsjs/go/models";
import { storyarcKeys } from "@/lib/queryKeys";

// useUpdateStoryArc: update story arc mutation.
// mutationFn imports wailsjs UpdateStoryArc directly (not useApp).
// Input {id, input}: input uses app.UpdateStoryArcInput (PUT semantics,
// full payload).
// The handler is responsible for setEditMode(null) + throwing the error
// (the side effects differ, so they are not placed in the mutation).
// onSuccess invalidates storyarcs: the arc name/type/importance/status
// changes enter the list.
// arc-nodes is not invalidated: arc fields do not affect the node cache
// (nodes only store a story_arc_id reference).
export function useUpdateStoryArc(novelId: number) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({
      id,
      input,
    }: {
      id: number;
      input: app.UpdateStoryArcInput;
    }) => UpdateStoryArc(novelId, id, input),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: storyarcKeys.list(novelId) });
    },
  });
}
