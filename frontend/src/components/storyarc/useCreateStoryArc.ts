import { useMutation, useQueryClient } from "@tanstack/react-query";
import { CreateStoryArc } from "@/lib/wailsjs/go/app/App";
import type { app } from "@/lib/wailsjs/go/models";
import { storyarcKeys } from "@/lib/queryKeys";

// useCreateStoryArc: create story arc mutation.
// mutationFn imports wailsjs CreateStoryArc directly (not useApp) and
// returns storyarc.StoryArc.
// The handler is responsible for setEditMode(null) + throwing the error
// (the side effects differ, so they are not placed in the mutation).
// onSuccess invalidates storyarcs: the new arc enters the list, and
// ArcList / ArcListView / StoryArcGraph stay in sync.
// arc-nodes is not invalidated: a newly created arc has no nodes, so the
// arc-nodes cache has no dirty data.
export function useCreateStoryArc(novelId: number) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: app.CreateStoryArcInput) =>
      CreateStoryArc(novelId, input),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: storyarcKeys.list(novelId) });
    },
  });
}
