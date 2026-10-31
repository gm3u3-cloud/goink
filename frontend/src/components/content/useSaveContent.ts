import { useMutation, useQueryClient } from "@tanstack/react-query";
import { SaveContent } from "@/lib/wailsjs/go/app/App";
import type { app } from "@/lib/wailsjs/go/models";
import { contentKeys, skillKeys } from "@/lib/queryKeys";

// useSaveContent: mutation to save file content.
// mutationFn imports wailsjs SaveContent directly (not useApp), with a
// single input argument (including novel_id + path + content).
// onSuccess invalidates contentKeys.detail(input.novel_id, input.path):
// keeps the useFileContent cache consistent so external reads of the same
// path do not get stale content (e.g. the file:changed handler, multi-tab
// same-path scenarios).
// The caller's doSave is responsible for updateTab(isDirty:false) +
// try/catch + toastError (tabs are local state and do not go into the query
// cache).
//
// 5.3 reuse: StyleView/PatternSessionView save skill/style content through
// this hook (generic, not tied to chapter semantics).
export function useSaveContent() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: app.SaveContentInput) => SaveContent(input),
    onSuccess: (_data, input) => {
      if (input.path.startsWith("~/.goink/skills/")) {
        void qc.invalidateQueries({
          predicate: ({ queryKey }) =>
            queryKey[0] === contentKeys.all[0] && queryKey[2] === input.path,
        });
        void qc.invalidateQueries({ queryKey: skillKeys.all });
      } else {
        void qc.invalidateQueries({
          queryKey: contentKeys.detail(input.novel_id, input.path),
        });
        if (input.path.startsWith("skills/"))
          void qc.invalidateQueries({
            queryKey: skillKeys.list(input.novel_id),
          });
      }
    },
  });
}
