import { useMutation, useQueryClient } from "@tanstack/react-query";
import { CreateNovel } from "@/lib/wailsjs/go/app/App";
import type { app } from "@/lib/wailsjs/go/models";
import { novelKeys } from "@/lib/queryKeys";

// useCreateNovel: create-novel mutation.
// mutationFn directly imports the wailsjs CreateNovel (no useApp); onSuccess
// invalidates novelKeys.all.
// The handler is responsible for switchToNovel + closing the UI (side effects
// vary, so they are not put in the mutation).
// The input type is app.CreateNovelInput: title is required, description/genre
// are optional.
export function useCreateNovel() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: app.CreateNovelInput) => CreateNovel(input),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: novelKeys.all });
    },
  });
}
