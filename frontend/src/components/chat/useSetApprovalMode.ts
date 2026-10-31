import { useMutation, useQueryClient } from "@tanstack/react-query";
import { SetApprovalMode } from "@/lib/wailsjs/go/app/App";
import { settingsKeys } from "@/lib/queryKeys";

// useSetApprovalMode: mutation to set the approval mode (persists to
// AppSettings.approval_mode).
// onSuccess invalidates settingsKeys.all — useSettings refetches the new
// approval_mode.
// Error handling is done by the caller with try/catch + toastError.
// 5.1 commit 4 addendum.
export function useSetApprovalMode() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (mode: string) => SetApprovalMode(mode),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: settingsKeys.all });
    },
  });
}
