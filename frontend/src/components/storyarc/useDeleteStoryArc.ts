import { useMutation, useQueryClient } from "@tanstack/react-query";
import { DeleteStoryArc } from "@/lib/wailsjs/go/app/App";
import { storyarcKeys, arcNodeKeys } from "@/lib/queryKeys";

// useDeleteStoryArc: delete narrative arc mutation.
// mutationFn imports wailsjs DeleteStoryArc directly (not useApp).
// Consumer: ArcListView.confirmDelete (the mutateAsync throw is caught by
// the handler's try/catch).
// The handler is responsible for setExpandedId(null) +
// setDeleteTarget(null) + the error toast (side effects are not placed in
// the mutation).
// onSuccess invalidates storyarcs + arc-nodes: the backend DeleteStoryArc
// transaction cascades to delete all of that arc's nodes, so both
// frontend query caches must refresh (ArcListView/ArcList's useStoryArcs +
// ArcListView/StoryArcGraph's useArcNodes both refetch, keeping the cache
// clean).
// max-chapter is not invalidated: deleting an arc does not affect the
// novel's max chapter number.
export function useDeleteStoryArc(novelId: number) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: number) => DeleteStoryArc(novelId, id),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: storyarcKeys.list(novelId) });
      qc.invalidateQueries({ queryKey: arcNodeKeys.list(novelId) });
    },
  });
}
