import { useMutation, useQueryClient } from "@tanstack/react-query";
import { SaveUserName } from "@/lib/wailsjs/go/app/App";
import { settingsKeys } from "@/lib/queryKeys";

// useSaveUserName: save-username mutation.
// mutationFn directly imports the wailsjs SaveUserName (no useApp).
// onSuccess invalidates settingsKeys.all — useProfileSettings auto-refetches
// to get the new user_name (settingsKeys.all shares the cache with chat;
// chat does not read user_name, so it is unaffected).
// Error handling is done by the caller with try/catch + setNameError
// (keeping the component-level inline error display).
// 5.7 commit 2.
export function useSaveUserName() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (name: string) => SaveUserName(name),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: settingsKeys.all });
    },
  });
}
