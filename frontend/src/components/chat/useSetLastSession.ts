import { useMutation, useQueryClient } from "@tanstack/react-query";
import { SetLastSession } from "@/lib/wailsjs/go/app/App";
import { settingsKeys } from "@/lib/queryKeys";

// useSetLastSession: mutation to set the last session ID (persists to
// AppSettings.last_session_id).
// onSuccess invalidates settingsKeys.all — useSettings refetches the new
// last_session_id, keeping the cache in sync with the backend.
// Error handling is done by the caller with try/catch + toastError.
// 5.1 commit 4 addendum.
export function useSetLastSession() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (sessionId: string) => SetLastSession(sessionId),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: settingsKeys.all });
    },
  });
}
