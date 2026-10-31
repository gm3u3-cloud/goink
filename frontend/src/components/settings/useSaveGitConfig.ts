import { useMutation, useQueryClient } from "@tanstack/react-query";
import { SaveGitConfig } from "@/lib/wailsjs/go/app/App";
import { settingsKeys } from "@/lib/queryKeys";

// useSaveGitConfig: mutation to save the Git commit author config.
// onSuccess invalidates settingsKeys.all — GetSettings returns the
// git_name/git_email fields, so after saving useSettings must refetch the
// new values; otherwise, when GeneralConfigTab is left and returned to, the
// useEffect back-fills gitName/gitEmail from the stale settingsQuery.data
// and shows the old values.
// chat/profile's useSettings/useProfileSettings refetch the same data (they
// do not read the git fields), so there are no side effects.
// Error handling is done by the caller with try/catch + inline gitError
// (keeps the component-level inline error display).
// 5.8 commit 2.
export function useSaveGitConfig() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: { gitName: string; gitEmail: string }) =>
      SaveGitConfig(input.gitName, input.gitEmail),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: settingsKeys.all });
    },
  });
}
