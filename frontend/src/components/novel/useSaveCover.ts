import { useMutation, useQueryClient } from "@tanstack/react-query";
import { SaveCover } from "@/lib/wailsjs/go/app/App";
import { novelKeys } from "@/lib/queryKeys";

// useSaveCover: save-novel-cover mutation.
// mutationFn directly imports the wailsjs SaveCover (no useApp); onSuccess
// invalidates novelKeys.all so useNovels refetches and gets the new cover
// URL.
// Error handling is done by the caller with try/catch + toastError
// (novel.coverSaveFailed).
// The input {novelId, cover} is destructured: novelId is the novel ID, cover
// is the cover byte array (Array<number>).
export function useSaveCover() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ novelId, cover }: { novelId: number; cover: number[] }) =>
      SaveCover(novelId, cover),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: novelKeys.all });
    },
  });
}
