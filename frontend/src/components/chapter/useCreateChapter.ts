import { useMutation, useQueryClient } from "@tanstack/react-query";
import { CreateChapter } from "@/lib/wailsjs/go/app/App";
import type { app } from "@/lib/wailsjs/go/models";
import { chapterKeys, maxChapterKeys } from "@/lib/queryKeys";

// useCreateChapter: create-chapter mutation.
// mutationFn directly imports wailsjs CreateChapter (no useApp); a single
// input arg (containing novel_id + title).
// onSuccess invalidates chapterKeys.list (refresh ChapterList) +
// maxChapterKeys.detail (required, a storyarc 4.3 leftover: after creating a
// chapter, the storyarc chapter window center windowCenter must be updated,
// otherwise the window is offset).
// The caller handleCreateChapter is responsible for
// setChapterTitle("")/setShowCreateChapter(false)/setCreateError + try/catch.
export function useCreateChapter(novelId: number) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: app.CreateChapterInput) => CreateChapter(input),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: chapterKeys.list(novelId) });
      qc.invalidateQueries({ queryKey: maxChapterKeys.detail(novelId) });
    },
  });
}
