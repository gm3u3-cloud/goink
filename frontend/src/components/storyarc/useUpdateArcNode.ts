import { useMutation, useQueryClient } from "@tanstack/react-query";
import { UpdateArcNode } from "@/lib/wailsjs/go/app/App";
import type { app } from "@/lib/wailsjs/go/models";
import { arcNodeKeys } from "@/lib/queryKeys";

// useUpdateArcNode: update arc node mutation.
// mutationFn imports wailsjs UpdateArcNode directly (not useApp).
// Input {id, input}: input uses app.UpdateArcNodeInput (PUT semantics,
// full payload).
// The handler is responsible for setEditMode(null)/setExpandedId +
// throwing the error (the side effects differ, so they are not placed in
// the mutation).
// onSuccess invalidates arc-nodes: the node field changes enter the list.
// storyarcs is not invalidated: node fields do not affect the arc list.
export function useUpdateArcNode(novelId: number) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({
      id,
      input,
    }: {
      id: number;
      input: app.UpdateArcNodeInput;
    }) => UpdateArcNode(novelId, id, input),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: arcNodeKeys.list(novelId) });
    },
  });
}
