import { useMutation, useQueryClient } from "@tanstack/react-query";
import { DeleteArcNode } from "@/lib/wailsjs/go/app/App";
import { arcNodeKeys } from "@/lib/queryKeys";

// useDeleteArcNode: delete arc node mutation.
// mutationFn imports wailsjs DeleteArcNode directly (not useApp).
// Consumer: ArcListView.confirmDelete (the mutateAsync throw is caught by
// the handler's try/catch).
// The handler is responsible for setExpandedId(null) +
// setDeleteTarget(null) + the error toast (side effects are not placed in
// the mutation).
// onSuccess only invalidates arc-nodes: deleting a node does not affect
// the arcs list (useStoryArcs) nor max-chapter.
// ArcListView/StoryArcGraph's useArcNodes both refetch, keeping the cache
// clean.
export function useDeleteArcNode(novelId: number) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: number) => DeleteArcNode(novelId, id),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: arcNodeKeys.list(novelId) });
    },
  });
}
