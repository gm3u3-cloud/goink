import { useMutation, useQueryClient } from "@tanstack/react-query";
import { DeleteSession } from "@/lib/wailsjs/go/app/App";

// useDeleteSession: session deletion mutation.
// onSuccess invalidates the full ["sessions"] prefix (including list +
// infiniteList), automatically refreshing all session list caches.
// No onError is attached — the caller does try/catch + toastError
// (following the convention: mutations do not attach onError).
export function useDeleteSession() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => DeleteSession(id),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["sessions"] });
    },
  });
}
