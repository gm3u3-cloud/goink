import { useMutation, useQueryClient } from "@tanstack/react-query";
import { UpdateNovel } from "@/lib/wailsjs/go/app/App";
import type { app } from "@/lib/wailsjs/go/models";
import { novelKeys } from "@/lib/queryKeys";

// useUpdateNovel: update-novel mutation.
// mutationFn directly imports the wailsjs UpdateNovel (no useApp); onSuccess
// invalidates novelKeys.all.
// Input {id, input}: input sends all fields back (PUT semantics, see
// 00-conventions §6).
// The handler is responsible for setEditingNovel(null) + throwing the error
// (side effects vary, so they are not put in the mutation).
export function useUpdateNovel() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, input }: { id: number; input: app.UpdateNovelInput }) =>
      UpdateNovel(id, input),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: novelKeys.all });
    },
  });
}
