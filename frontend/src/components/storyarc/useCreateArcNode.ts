import { useMutation, useQueryClient } from "@tanstack/react-query";
import { CreateArcNode } from "@/lib/wailsjs/go/app/App";
import type { app } from "@/lib/wailsjs/go/models";
import { arcNodeKeys } from "@/lib/queryKeys";

// useCreateArcNode: create arc node mutation.
// mutationFn imports wailsjs CreateArcNode directly (not useApp) and
// returns storyarc.ArcNode.
// The handler uses the returned created.id for setExpandedId (the side
// effects differ, so they are not placed in the mutation).
// onSuccess invalidates arc-nodes: the new node enters the list, and
// ArcListView / StoryArcGraph stay in sync.
// storyarcs is not invalidated: a newly created node does not affect the
// arc list.
export function useCreateArcNode(novelId: number) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: app.CreateArcNodeInput) =>
      CreateArcNode(novelId, input),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: arcNodeKeys.list(novelId) });
    },
  });
}
