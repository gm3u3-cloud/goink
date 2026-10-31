import { useMutation, useQueryClient } from "@tanstack/react-query";
import { SetReasoningEffort } from "@/lib/wailsjs/go/app/App";
import { settingsKeys } from "@/lib/queryKeys";

// useSetReasoningEffort: mutation to set the reasoning effort (persists to
// AppSettings.reasoning_effort).
// onSuccess invalidates settingsKeys.all — useSettings refetches the new
// reasoning_effort.
// Error handling is done by the caller with try/catch + toastError.
// 5.1 commit 4 addendum.
export function useSetReasoningEffort() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (effort: string) => SetReasoningEffort(effort),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: settingsKeys.all });
    },
  });
}
