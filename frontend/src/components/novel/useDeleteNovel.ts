import { useMutation, useQueryClient } from "@tanstack/react-query";
import { DeleteNovel } from "@/lib/wailsjs/go/app/App";
import { novelKeys } from "@/lib/queryKeys";

// useDeleteNovel: delete-novel mutation.
// mutationFn directly imports the wailsjs DeleteNovel (no useApp); onSuccess
// invalidates novelKeys.all.
// The handler is responsible for setDeletingNovel(null) + throwing the error
// (side effects vary, so they are not put in the mutation).
// When deleting the current activeNovelId, the auto-select-novel effect
// (WorkspaceView) takes over after the refetch and selects the first one.
export function useDeleteNovel() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: number) => DeleteNovel(id),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: novelKeys.all });
    },
  });
}
