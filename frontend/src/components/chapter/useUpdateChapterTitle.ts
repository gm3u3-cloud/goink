import { useMutation, useQueryClient } from "@tanstack/react-query";
import { UpdateChapterTitle } from "@/lib/wailsjs/go/app/App";
import { chapterKeys } from "@/lib/queryKeys";

// useUpdateChapterTitle: update-chapter-title mutation.
// mutationFn directly imports wailsjs UpdateChapterTitle (no useApp); 3 args
// (novelId, chapterID, title).
// §6 full payload: the input {chapterID, title} is passed in full
// (UpdateChapterTitle takes exactly these two, no patch semantics).
// onSuccess invalidates chapterKeys.list (refresh the ChapterList title).
// The caller commitEdit is responsible for setEditingId(null) + try/catch +
// toastError.
export function useUpdateChapterTitle(novelId: number) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ chapterID, title }: { chapterID: number; title: string }) =>
      UpdateChapterTitle(novelId, chapterID, title),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: chapterKeys.list(novelId) });
    },
  });
}
